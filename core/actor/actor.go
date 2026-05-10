package actor

// Type identifies the kind of request initiator (generic identity, not domain model).
type Type string

const (
	TypeUser      Type = "user"
	TypeSystem    Type = "system"
	TypeAnonymous Type = "anonymous"
	TypeService   Type = "service"
)

// Actor 表示请求发起者的最小调用源拓扑通用语：仅含 ID/Type/DisplayName 三件套。
//
// 协议特定字段（OAuth/OIDC：Email/Subject/ClientID/Realm/Roles/Scopes 与开放扩展袋
// Attrs）不在该接口暴露；如业务需要，请由业务自定义 ctx 信道（如业务自家 IAM 包提供的
// WithUserInfo / UserInfoFromContext）承载，servora 主仓不预设 OIDC ctx 信道。
type Actor interface {
	ID() string
	Type() Type
	DisplayName() string
}
