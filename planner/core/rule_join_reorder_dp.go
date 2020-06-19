// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/parser/ast"
)

type joinReorderDPSolver struct {
	*baseSingleGroupJoinOrderSolver
	newJoin             func(lChild, rChild LogicalPlan, eqConds []*expression.ScalarFunction, otherConds []expression.Expression) LogicalPlan
	joinGroupNonEqEdges []joinGroupNonEqEdge // dp的结果
}

// 这里就是处理带有on条件的情况的
type joinGroupEqEdge struct {
	nodeIDs []int
	edge    *expression.ScalarFunction
}

// 这里就是处理`select * from t, t1, t2`这种情况的
type joinGroupNonEqEdge struct {
	nodeIDs    []int
	nodeIDMask uint
	expr       expression.Expression // 这个还不懂有什么用
}

func (s *joinReorderDPSolver) solve(joinGroup []LogicalPlan, eqConds []expression.Expression) (LogicalPlan, error) {
	// TODO: You need to implement the join reorder algo based on DP.

	// The pseudo code can be found in README.
	// And there's some common struct and method like `baseNodeCumCost`, `calcJoinCumCost` you can use in `rule_join_reorder.go`.
	// Also, you can take a look at `rule_join_reorder_greedy.go`, this file implement the join reorder algo based on greedy algorithm.
	// You'll see some common usages in the greedy version.

	// Note that the join tree may be disconnected. i.e. You need to consider the case `select * from t, t1, t2`.

	// 第一步 dp的初始化
	for i, jg := range joinGroup {

	}
	// 第二步 补全dp数组
	// 第三步 取结果

	return nil, errors.Errorf("unimplemented")
}

// newJoinWithEdge 用左右2个节点生成一个新的join计划
// edges目前还不懂确切的作用，看起来是join的条件表达式+节点编号
// 这一步对应的应该是readme上面的f[group] = join(f[sub], f[group ^ sub])操作
// 例如
// f[0x001] = a
// f[0x010] = b
// f[0x100] = c
// f[0x011] = join(a, b)
// f[0x101] = join(a, c)
// f[0x110] = join(b, c)
// f[0x111] = min{join(f[0x001], f[0x110]),join(f[0x010], f[0x101]),join(f[0x100], f[0x011])}
// 那可以推测edges应该是join的条件，otherConds是where条件
func (s *joinReorderDPSolver) newJoinWithEdge(leftPlan, rightPlan LogicalPlan, edges []joinGroupEqEdge, otherConds []expression.Expression) (LogicalPlan, error) {
	var eqConds []*expression.ScalarFunction
	for _, edge := range edges {
		lCol := edge.edge.GetArgs()[0].(*expression.Column)
		rCol := edge.edge.GetArgs()[1].(*expression.Column)
		if leftPlan.Schema().Contains(lCol) {
			eqConds = append(eqConds, edge.edge)
		} else {
			newSf := expression.NewFunctionInternal(s.ctx, ast.EQ, edge.edge.GetType(), rCol, lCol).(*expression.ScalarFunction)
			eqConds = append(eqConds, newSf)
		}
	}
	// 单元测试时，这里返回的是mockjoin，它的统计信息是写死在执行计划里面的，recursiveDeriveStats()负责写统计信息
	join := s.newJoin(leftPlan, rightPlan, eqConds, otherConds)
	_, err := join.recursiveDeriveStats()
	return join, err
}

// Make cartesian join as bushy tree.
// makeBushyJoin build bushy tree for the nodes which have no equal condition to connect them.
// 这里就是处理`select * from t, t1, t2`这种情况的
func (s *joinReorderDPSolver) makeBushyJoin(cartesianJoinGroup []LogicalPlan, otherConds []expression.Expression) LogicalPlan {
	for len(cartesianJoinGroup) > 1 {
		resultJoinGroup := make([]LogicalPlan, 0, len(cartesianJoinGroup))
		for i := 0; i < len(cartesianJoinGroup); i += 2 {
			if i+1 == len(cartesianJoinGroup) {
				resultJoinGroup = append(resultJoinGroup, cartesianJoinGroup[i])
				break
			}
			// TODO:Since the other condition may involve more than two tables, e.g. t1.a = t2.b+t3.c.
			//  So We'll need a extra stage to deal with it.
			// Currently, we just add it when building cartesianJoinGroup.
			mergedSchema := expression.MergeSchema(cartesianJoinGroup[i].Schema(), cartesianJoinGroup[i+1].Schema())
			var usedOtherConds []expression.Expression
			otherConds, usedOtherConds = expression.FilterOutInPlace(otherConds, func(expr expression.Expression) bool {
				return expression.ExprFromSchema(expr, mergedSchema)
			})
			resultJoinGroup = append(resultJoinGroup, s.newJoin(cartesianJoinGroup[i], cartesianJoinGroup[i+1], nil, usedOtherConds))
		}
		cartesianJoinGroup = resultJoinGroup
	}
	return cartesianJoinGroup[0]
}

// findNodeIndexInGroup 返回col在group的位置
func findNodeIndexInGroup(group []LogicalPlan, col *expression.Column) (int, error) {
	for i, plan := range group {
		if plan.Schema().Contains(col) {
			return i, nil
		}
	}
	return -1, ErrUnknownColumn.GenWithStackByArgs(col, "JOIN REORDER RULE")
}
