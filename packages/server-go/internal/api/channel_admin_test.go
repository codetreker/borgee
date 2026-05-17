// Package api_test — chn_11_member_admin_test.go: CHN-11 0-server-prod
// grep 检查守门 (CHN-11 仅 client SPA — server-side POST/DELETE/GET
// /channels/:id/members CHN-1 #276 既有 path byte-identical 不变).
//
// Pins:
//
//	REG-CHN11-001 TestChannelAdmin_NoSchemaChange (filepath.Walk migrations/)
//	REG-CHN11-002 TestChannelAdmin_NoServerProductionCode (grep 检查 `chn_11`
//	               在 internal/api/*.go 非 _test.go 0 hit)
//	REG-CHN11-003 TestCHN_HandlersByteIdentical (handleAddMember +
//	               handleRemoveMember block grep 检查 `chn_11` 0 hit)
//	REG-CHN11-004 TestCHN_NoMemberAdminQueue (AST 锁链延伸第 19 处)
//	REG-CHN11-005 TestCHN_NoAdminMembersPath
package api_test
