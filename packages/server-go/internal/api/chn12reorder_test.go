// Package api_test — chn_12_reorder_test.go: CHN-12 0-server-prod 反向
// 源码扫描守门 (CHN-12 仅 client SPA dnd_position.ts + ChannelDragHandle 既有
// SortableChannelItem; server-side PUT /api/v1/me/layout CHN-3.2 #357
// 既有 path 字节级一致 不变).
//
// 覆盖的检查点:
//
//	REG-CHN12-001 TestChn12reorder_NoSchemaChange (filepath.Walk migrations/)
//	REG-CHN12-002 TestChn12reorder_NoServerProductionCode (grep 检查 `chn_12`
//	               在 internal/api/*.go 非 _test.go 0 hit)
//	REG-CHN12-003 TestCHN_HandlerByteIdentical (handlePutMyLayout block
//	               grep 检查 `chn_12` 0 hit)
//	REG-CHN12-004 TestCHN_NoReorderQueue (AST 对齐检查第 20 处)
//	REG-CHN12-005 TestCHN_NoAdminLayoutPath
package api_test
