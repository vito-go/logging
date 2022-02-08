package logging

import _ "embed"

//go:embed login.html
// jumpPath loginPath
var loginHtml string

//go:embed log.html
// title  logPushPath
// tokenFailed 标记token是否校验失败.用于js提示. 只有在登录失败时候提示
var logHtml string

//go:embed tid.html
// tieSearchPath tid搜索路径
var tidHtml string
