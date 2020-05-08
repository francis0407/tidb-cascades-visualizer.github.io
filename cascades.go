package main

import (
	"bytes"
	"fmt"
	svg "github.com/ajstarks/svgo"
	"github.com/pingcap/tidb/planner/memo"
	"sort"
)

// Because the snapshot may be deserialized from json, we have to
// set the children `pointer`s for the GroupExpr.
func setGroupExprChildren(snapshot *memo.MemoSnapshot) {
	groups := snapshot.Groups
	for _, group := range groups {
		for _, expr := range group.Exprs {
			expr.Children = make([]*memo.GroupInfo, len(expr.ChildrenIDs))
			for i, childID := range expr.ChildrenIDs {
				expr.Children[i] = groups[childID]
			}
		}
	}
}

type MemoRelativeGrid [][]*memo.GroupInfo

func GetRelativeGrid(groups []*memo.GroupInfo) MemoRelativeGrid {
	// 1. Use a topological order to split the memo to multiple levels.
	indegreeMap := make(map[*memo.GroupInfo]int)
	for _, group := range groups {
		indegreeMap[group] = 0
	}
	for _, group := range groups {
		for _, expr := range group.Exprs {
			for _, child := range expr.Children {
					indegreeMap[child] ++
			}
		}
	}
	sortedMemo := topologicalSortMemo(indegreeMap)

	// 2. Sort groups by ID in each level.
	// TODO: 'Sort by ID' is NOT the best to way to arrange
	// groups in the same level, should use a more advanced algorithm.
	for _, level := range sortedMemo {
		sort.Slice(level, func(i, j int) bool {
			return level[i].ID < level[j].ID
		})
	}

	return sortedMemo
}

func topologicalSortMemo(indegree map[*memo.GroupInfo]int) [][]*memo.GroupInfo {
	if len(indegree) == 0 {
		return nil
	}
	candidates := make([]*memo.GroupInfo, 0)
	for group, degree := range indegree {
		if degree == 0 {
			candidates = append(candidates, group)
		}
	}
	for _, group := range candidates {
		delete(indegree, group)
		for _, expr := range group.Exprs {
			for _, child := range expr.Children {
				indegree[child] --
			}
		}
	}
	result := [][]*memo.GroupInfo{candidates}
	return append(result, topologicalSortMemo(indegree)...)
}

type BlockBase struct {
	LeftOffset int
	TopOffset  int
	Height     int
	Width      int
}

type GroupBlock struct {
	*memo.GroupInfo
	PropBlock *GroupPropBlock
	ExprBlocks []*GroupExprBlock

	BlockBase
}

type GroupPropBlock struct {
	BlockBase
}

type GroupExprBlock struct {
	*memo.GroupExprInfo
	ExprInfoBlock *GroupExprInfoBlock
	BlockBase
}

type GroupExprInfoBlock struct {
	BlockBase
}

func (g *GroupBlock) AppendSvgObject(canvas *svg.SVG) {
	canvas.Rect(g.LeftOffset, g.TopOffset, g.Width, g.Height,"fill:GhostWhite;stroke:black;stroke-width:3;")
	textStyle := fmt.Sprintf("text-anchor:middle;dominant-baseline:middle;font-size:%dpx", FontSize)
	textContent := fmt.Sprintf("Group%d", g.GroupInfo.ID)
	canvas.Text(g.LeftOffset + GroupExprLeftBaseOffset + GroupTextWidth/2, g.TopOffset + GroupHeight/2, textContent, textStyle)
}

func (expr *GroupExprBlock) AppendSvgObject(canvas *svg.SVG, id int) {
	canvas.Rect(
		expr.LeftOffset,
		expr.TopOffset,
		expr.Width,
		expr.Height,
		fmt.Sprintf("id=GroupExpr-%d", id),
		"class=GroupExpr",
		"fill:GhostWhite;stroke:black;stroke-width:3;")
	textStyle := fmt.Sprintf("text-anchor:middle;dominant-baseline:middle;font-size:%dpx", FontSize)
	canvas.Text(expr.LeftOffset + expr.Width/2, expr.TopOffset + expr.Height/2, expr.GroupExprInfo.Operand, textStyle)
}

func (expr *GroupExprBlock) AppendPointerToGroup(canvas *svg.SVG, groups map[int]*GroupBlock) {
	// TODO: it would be better to use an `Arrow` here.
	for _, child := range expr.Children {
		childBlock := groups[child.ID]
		canvas.Line(
			expr.LeftOffset + expr.Width / 2,
			expr.TopOffset + expr.Height,
			childBlock.LeftOffset + childBlock.Width / 2,
			childBlock.TopOffset,
			"stroke: black;stroke-width:3;",
		)
	}
}

func (expr *GroupExprBlock) AppendExplainInfo(canvas *svg.SVG, id int) {
	info := expr.GroupExprInfo.ExprInfo
	textStyle := fmt.Sprintf("text-anchor:middle;dominant-baseline:middle;font-size:%dpx;fill:blue;display:none;", FontSize)
	canvas.Text(
		expr.LeftOffset + expr.Width/2,
		expr.TopOffset - GroupExprTopBaseOffset/2,
		info,
		fmt.Sprintf("class=ExplainInfo-%d", id),
		textStyle,
	)
}

const (
	GroupHeight = 150
	GroupTextWidth = 50
	GroupTopBaseOffset = 20
	GroupLeftBaseOffset = 20

	GroupExprHeight = 80
	GroupExprWidth = 200

	GroupExprTopBaseOffset = (GroupHeight - GroupExprHeight) / 2
	GroupExprLeftBaseOffset = 20

	FontSize = 20 // px
)

func BuildGroupBlock(level int, leftOffset int, group *memo.GroupInfo) (nextOffset int, groupBlock *GroupBlock) {
	groupBlock = &GroupBlock{
		GroupInfo : group,
		PropBlock : nil,
		ExprBlocks: make([]*GroupExprBlock, 0, len(group.Exprs)),
		BlockBase : BlockBase{
			LeftOffset: leftOffset + GroupLeftBaseOffset,
			TopOffset:  level * (GroupTopBaseOffset + GroupHeight) + GroupTopBaseOffset,
			Height:     GroupHeight,
			/* GroupExprBaseOffset + GroupText + [GroupExprs] + GroupExprBaseOffset*/
			Width:      GroupExprLeftBaseOffset + GroupTextWidth + len(group.Exprs) * (GroupExprWidth + GroupExprLeftBaseOffset) + GroupExprLeftBaseOffset, // width will be set below.
		},
	}

	// TODO: Set PropBlock.

	for i, expr := range group.Exprs {
		exprBlock := &GroupExprBlock{
			GroupExprInfo: expr,
			ExprInfoBlock: nil,
			BlockBase:     BlockBase{
				LeftOffset: groupBlock.LeftOffset +
					GroupExprLeftBaseOffset + GroupTextWidth +
					i * (GroupExprWidth + GroupExprLeftBaseOffset) + GroupExprLeftBaseOffset,
				TopOffset:  groupBlock.TopOffset + GroupExprTopBaseOffset,
				Height:     GroupExprHeight,
				Width:      GroupExprWidth,
			},
		}
		// TODO: Set ExprInfoBlock.

		groupBlock.ExprBlocks = append(groupBlock.ExprBlocks, exprBlock)
	}

	return groupBlock.LeftOffset + groupBlock.Width, groupBlock
}

func GetGroupBlocks(grid MemoRelativeGrid) [][]*GroupBlock {
	levels := make([][]*GroupBlock, 0, len(grid))
	for groupLevel := range grid {
		blockLevel := make([]*GroupBlock, 0, len(grid[groupLevel]))
		leftOffset := 0
		for _, groupInfo := range grid[groupLevel] {
			nextOffset, groupBlock := BuildGroupBlock(groupLevel, leftOffset, groupInfo)
			blockLevel = append(blockLevel, groupBlock)
			leftOffset = nextOffset
		}
		levels = append(levels, blockLevel)
	}
	return levels
}

func ConvertBlocks2SVG(groups [][]*GroupBlock) (*svg.SVG, *bytes.Buffer) {
	buffer := bytes.NewBuffer([]byte{})
	canvas := svg.New(buffer)
	// TODO: this should be set according to the memo.
	canvas.Start(10000, 10000)

	exprCount := 0
	for _, levels := range groups {
		for _, group := range levels {
			group.AppendSvgObject(canvas)
			for _, expr := range group.ExprBlocks {
				expr.AppendSvgObject(canvas, exprCount)
				exprCount++
			}
		}
	}
	// Add pointer lines.
	// 1. Collect groupBlocks
	// 2. Create lines.
	groupIDMap := make(map[int]*GroupBlock)
	for _, levels := range groups {
		for _, group := range levels {
			groupIDMap[group.GroupInfo.ID] = group
		}
	}
	for _, levels := range groups {
		for _, group := range levels {
			for _, expr := range group.ExprBlocks {
				expr.AppendPointerToGroup(canvas, groupIDMap)
			}
		}
	}

	// Add ExplainInfo.
	exprCount = 0
	for _, levels := range groups {
		for _, group := range levels {
			for _, expr := range group.ExprBlocks {
				expr.AppendExplainInfo(canvas, exprCount)
				exprCount++
			}
		}
	}
	canvas.End()
	return canvas, buffer
}

type SnapShotSVG struct {
	Rule string `json:"rule"`
	SVG string `json:"svg"`
	snapshot *memo.MemoSnapshot `json:"-"`
}

func ConvertMemoSnapshot2SVG(snapshot *memo.MemoSnapshot) *SnapShotSVG {
	setGroupExprChildren(snapshot)
	// 1. Calculate Relative Position.
	relativeGrid := GetRelativeGrid(snapshot.Groups)
	// 2. Set Absolute Position(size).
	groupBlocks := GetGroupBlocks(relativeGrid)
	// 3. Covert groupBlocks to SVG objects.
	_, buffer := ConvertBlocks2SVG(groupBlocks)
	return &SnapShotSVG{
		Rule: snapshot.RuleName,
		SVG: buffer.String(),
		snapshot: snapshot,
	}
}





