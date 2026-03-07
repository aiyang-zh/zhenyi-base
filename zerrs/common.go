package zerrs

// 通用 sentinel 错误。均为 *TypedError，可同时用于 errors.Is 和 [IsType]。
var (
	// ErrInvalidParameter 无效参数错误
	ErrInvalidParameter = New(ErrTypeValidation, "invalid parameter")

	// ErrTimeout 超时错误
	ErrTimeout = New(ErrTypeTimeout, "operation timeout")

	// ErrNotFound 资源未找到错误
	ErrNotFound = New(ErrTypeNotFound, "resource not found")

	// ErrAlreadyExists 资源已存在错误
	ErrAlreadyExists = New(ErrTypeAlreadyExists, "resource already exists")

	// ErrPermissionDenied 权限拒绝错误
	ErrPermissionDenied = New(ErrTypePermission, "permission denied")

	// ErrInternal 内部错误
	ErrInternal = New(ErrTypeInternal, "internal error")

	// ErrUnknown 未知错误
	ErrUnknown = New(ErrTypeUnknown, "unknown error")
)

// 网络与连接相关 sentinel 错误。
var (
	// ErrConnectionClosed 连接已关闭
	ErrConnectionClosed = New(ErrTypeConnection, "connection closed")

	// ErrConnectionFailed 连接失败
	ErrConnectionFailed = New(ErrTypeConnection, "connection failed")

	// ErrConnectionTimeout 连接超时
	ErrConnectionTimeout = New(ErrTypeTimeout, "connection timeout")

	// ErrNetworkUnreachable 网络不可达
	ErrNetworkUnreachable = New(ErrTypeNetwork, "network unreachable")

	// ErrReadTimeout 读超时
	ErrReadTimeout = New(ErrTypeTimeout, "read timeout")

	// ErrWriteTimeout 写超时
	ErrWriteTimeout = New(ErrTypeTimeout, "write timeout")
)

// Actor 相关 sentinel 错误。
var (
	// ErrActorNotFound Actor 未找到
	ErrActorNotFound = New(ErrTypeActor, "actor not found")

	// ErrActorStopped Actor 已停止
	ErrActorStopped = New(ErrTypeActor, "actor stopped")

	// ErrActorBusy Actor 繁忙
	ErrActorBusy = New(ErrTypeActor, "actor busy")

	// ErrActorTimeout Actor 超时
	ErrActorTimeout = New(ErrTypeTimeout, "actor timeout")

	// ErrActorAlreadyExists Actor 已存在
	ErrActorAlreadyExists = New(ErrTypeAlreadyExists, "actor already exists")
)

// RPC 相关 sentinel 错误。
var (
	// ErrRPCTimeout RPC 超时
	ErrRPCTimeout = New(ErrTypeTimeout, "rpc timeout")

	// ErrRPCFailed RPC 失败
	ErrRPCFailed = New(ErrTypeRPC, "rpc failed")

	// ErrRPCNotFound RPC 方法未找到
	ErrRPCNotFound = New(ErrTypeNotFound, "rpc method not found")

	// ErrRPCInvalidRequest RPC 请求无效
	ErrRPCInvalidRequest = New(ErrTypeValidation, "rpc invalid request")

	// ErrRPCInvalidResponse RPC 响应无效
	ErrRPCInvalidResponse = New(ErrTypeValidation, "rpc invalid response")
)

// 数据库相关 sentinel 错误。
var (
	// ErrDatabaseTimeout 数据库超时
	ErrDatabaseTimeout = New(ErrTypeTimeout, "database timeout")

	// ErrDatabaseConnectionFailed 数据库连接失败
	ErrDatabaseConnectionFailed = New(ErrTypeDatabase, "database connection failed")

	// ErrDatabaseQueryFailed 数据库查询失败
	ErrDatabaseQueryFailed = New(ErrTypeDatabase, "database query failed")

	// ErrDatabaseNotFound 数据库记录未找到
	ErrDatabaseNotFound = New(ErrTypeNotFound, "database record not found")

	// ErrDatabaseDuplicateKey 数据库重复键
	ErrDatabaseDuplicateKey = New(ErrTypeAlreadyExists, "database duplicate key")
)

// 配置相关 sentinel 错误。
var (
	// ErrConfigNotFound 配置未找到
	ErrConfigNotFound = New(ErrTypeNotFound, "config not found")

	// ErrConfigInvalid 配置无效
	ErrConfigInvalid = New(ErrTypeValidation, "config invalid")

	// ErrConfigLoadFailed 配置加载失败
	ErrConfigLoadFailed = New(ErrTypeConfig, "config load failed")

	// ErrConfigParseFailed 配置解析失败
	ErrConfigParseFailed = New(ErrTypeConfig, "config parse failed")
)

// 验证相关 sentinel 错误。
var (
	// ErrValidationFailed 验证失败
	ErrValidationFailed = New(ErrTypeValidation, "validation failed")

	// ErrInvalidFormat 格式无效
	ErrInvalidFormat = New(ErrTypeValidation, "invalid format")

	// ErrInvalidValue 值无效
	ErrInvalidValue = New(ErrTypeValidation, "invalid value")

	// ErrInvalidType 类型无效
	ErrInvalidType = New(ErrTypeValidation, "invalid type")

	// ErrOutOfRange 超出范围
	ErrOutOfRange = New(ErrTypeValidation, "out of range")
)

// InvalidParameterf 创建 VALIDATION 类型的参数错误，自动添加 "invalid parameter: " 前缀。
func InvalidParameterf(format string, args ...interface{}) error {
	return Newf(ErrTypeValidation, "invalid parameter: "+format, args...)
}

// Timeoutf 创建 TIMEOUT 类型的格式化错误。
func Timeoutf(format string, args ...interface{}) error {
	return Newf(ErrTypeTimeout, format, args...)
}

// NotFoundf 创建 NOT_FOUND 类型的格式化错误。
func NotFoundf(format string, args ...interface{}) error {
	return Newf(ErrTypeNotFound, format, args...)
}

// AlreadyExistsf 创建 ALREADY_EXISTS 类型的格式化错误。
func AlreadyExistsf(format string, args ...interface{}) error {
	return Newf(ErrTypeAlreadyExists, format, args...)
}

// Internalf 创建 INTERNAL 类型的格式化错误，自动捕获堆栈。
func Internalf(format string, args ...interface{}) error {
	return WithStackf(ErrTypeInternal, format, args...)
}

// Networkf 创建 NETWORK 类型的格式化错误。
func Networkf(format string, args ...interface{}) error {
	return Newf(ErrTypeNetwork, format, args...)
}

// Databasef 创建 DATABASE 类型的格式化错误。
func Databasef(format string, args ...interface{}) error {
	return Newf(ErrTypeDatabase, format, args...)
}

// Actorf 创建 ACTOR 类型的格式化错误。
func Actorf(format string, args ...interface{}) error {
	return Newf(ErrTypeActor, format, args...)
}

// RPCf 创建 RPC 类型的格式化错误。
func RPCf(format string, args ...interface{}) error {
	return Newf(ErrTypeRPC, format, args...)
}

// Configf 创建 CONFIG 类型的格式化错误。
func Configf(format string, args ...interface{}) error {
	return Newf(ErrTypeConfig, format, args...)
}
