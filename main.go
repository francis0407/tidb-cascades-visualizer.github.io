package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/pingcap/parser"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/planner/cascades"
	plannercore "github.com/pingcap/tidb/planner/core"
	"github.com/pingcap/tidb/planner/memo"
	"github.com/pingcap/tidb/session"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
)

var visualizer = NewCascadesVisualizer()

func main() {
	println("hello")
	js.Global().Set("GenerateSVGForSQL", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		res, err := visualizer.GenerateOptimizationProcessSVGForSQL(args[0].String())
		if err != nil {
			println(err.Error())
			args[1].Invoke(err.Error())
			return nil
		}
		jsonRes, err := json.Marshal(res)
		if err != nil {
			args[1].Invoke(err.Error())
			return nil
		}
		return string(jsonRes)
	}))
	select {}
	////visualizer.ExecuteSQL("use test")
	////visualizer.ExecuteSQL("create table t1 (a int, b int)")
	//res, err := visualizer.GenerateOptimizationProcessSVGForSQL("select a, b from t where a > 10")
	//if err != nil {
	//	println("error")
	//	return
	//}
	//for _, snapshot := range res {
	//	j, _ := json.Marshal(snapshot.snapshot)
	//	println(string(j))
	//	println(snapshot.SVG)
	//}

}

type CascadesVisualizer struct {
	*parser.Parser
	store     kv.Storage
	ctx       session.Session
	is        infoschema.InfoSchema
	optimizer *cascades.Optimizer
}

func NewCascadesVisualizer() *CascadesVisualizer {
	cv := &CascadesVisualizer{}
	//cv.store, _ = mockstore.NewMockTikvStore()
	//cv.ctx, _ = session.CreateSession4Test(cv.store)
	cv.is = infoschema.MockInfoSchema([]*model.TableInfo{plannercore.MockSignedTable()})
	cv.Parser = parser.New()
	cv.optimizer = cascades.NewOptimizer()
	cv.Parser.EnableWindowFunc(true)
	return cv
}

//func (cv *CascadesVisualizer) ExecuteSQL(sql string) ([]sqlexec.RecordSet, error){
//	return cv.ctx.Execute(context.Background(), sql)
//}

func (cv *CascadesVisualizer) GenerateOptimizationProcessSVGForSQL(sql string) ([]*SnapShotSVG, error){
	sctx := plannercore.MockContext()
	tracer := &stmtctx.CascadesTracer{}
	sctx.GetSessionVars().StmtCtx.CascadesTracer = tracer
	stmt, err := cv.ParseOneStmt(sql, "", "")
	if err != nil {
		return nil, err
	}
	p, _, err := plannercore.BuildLogicalPlan(context.Background(), sctx, stmt, cv.is)
	if err != nil {
		return nil, err
	}
	logic, ok := p.(plannercore.LogicalPlan)
	if !ok {
		return nil, fmt.Errorf("cannot generate LogicalPlan for SQL: %s", sql)
	}
	_, _, err = cv.optimizer.FindBestPlan(sctx, logic)
	if err != nil {
		return nil, err
	}

	result := make([]*SnapShotSVG, 0, len(tracer.MemoSnapshots))
	for _, s := range tracer.MemoSnapshots {
		snapshot, ok := s.(*memo.MemoSnapshot)
		if !ok {
			return nil, fmt.Errorf("tracer has unknown snapshot")
		}
		result = append(result, ConvertMemoSnapshot2SVG(snapshot))
	}
	return result, nil
}