package response

import (
	"strings"

	"github.com/gin-gonic/gin"
)

type Body struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(200, Body{Code: 0, Message: "ok", Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(201, Body{Code: 0, Message: "created", Data: data})
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, Body{Code: status, Message: localizeMessage(message)})
}

func localizeMessage(message string) string {
	messages := map[string]string{
		"account pending approval":               "账号正在审核中，请耐心等待管理员开通",
		"email already exists":                   "该邮箱已存在，请更换邮箱或直接登录",
		"email already registered":               "该邮箱已注册，请直接登录",
		"email not verified":                     "邮箱尚未完成验证，请先通过邮箱验证后再登录",
		"failed to approve order":                "订单审核失败，请稍后重试",
		"failed to create api key":               "API Key 创建失败，请稍后重试",
		"failed to create captcha":               "安全验证生成失败，请刷新后重试",
		"failed to create email code":            "邮箱验证码生成失败，请稍后重试",
		"failed to create order":                 "订单创建失败，请稍后重试",
		"failed to create plan":                  "套餐创建失败，请检查填写内容",
		"failed to delete plan":                  "套餐删除失败，请稍后重试",
		"failed to delete user":                  "用户删除失败，请稍后重试",
		"failed to disable api key":              "API Key 禁用失败，请稍后重试",
		"failed to generate api key":             "API Key 生成失败，请稍后重试",
		"failed to generate token":               "登录凭证生成失败，请稍后重试",
		"failed to hash password":                "密码处理失败，请稍后重试",
		"failed to reject order":                 "订单拒绝失败，请稍后重试",
		"failed to save email code":              "邮箱验证码保存失败，请稍后重试",
		"failed to update plan":                  "套餐更新失败，请检查填写内容",
		"failed to update settings":              "系统设置保存失败，请稍后重试",
		"failed to update user":                  "用户更新失败，请稍后重试",
		"invalid api key":                        "API Key 无效，请检查后重试",
		"invalid authorization token":            "登录状态已失效，请重新登录",
		"invalid credentials":                    "邮箱或密码不正确，请检查后重试",
		"invalid email verification code":        "邮箱验证码不正确或已过期",
		"invalid slide captcha":                  "安全验证未通过，请重新拖动滑块",
		"no active upstream account bound":       "当前账号尚未绑定可用上游通道，请联系管理员开通",
		"order already reviewed":                 "该订单已审核，请勿重复操作",
		"order not found":                        "订单不存在或已被删除",
		"plan not found":                         "套餐不存在或已下架",
		"subscription expired":                   "订阅已到期，请续费后继续使用",
		"subscription quota exceeded":            "本周美元额度已用完，请升级或续费后继续使用",
		"user disabled":                          "账号已被禁用，请联系管理员",
		"user is not approved":                   "账号尚未通过审核，请等待管理员开通",
		"user not found":                         "账号不存在，请重新登录",
		"password must be at least 8 characters": "密码至少需要 8 位",
	}
	if localized, ok := messages[message]; ok {
		return localized
	}
	if strings.Contains(message, "Field validation") {
		return "请检查表单必填项和格式是否正确"
	}
	if strings.HasPrefix(message, "failed to send email:") {
		return "邮件发送失败，请检查 SMTP 配置后重试"
	}
	return message
}
