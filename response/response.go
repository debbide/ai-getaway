package response

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

type Body struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(200, Body{Code: 0, Message: "ok", Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(201, Body{Code: 0, Message: "created", Data: data})
}

func Error(c *gin.Context, status int, message string) {
	localized := localizeMessage(message)
	c.JSON(status, Body{Code: status, Message: localized, Error: localized})
}

func localizeMessage(message string) string {
	if message == "令牌额度耗尽" {
		return message
	}
	switch message {
	case "missing api key":
		return "缺少 API Key"
	case "invalid api key":
		return "API Key 无效，请检查后重试"
	case "user is not approved", "account pending approval":
		return "账号尚未通过审核，请等待管理员开通"
	case "subscription expired":
		return "订阅已到期，请续费后继续使用"
	case "no active subscription assigned":
		return "当前账号未开通可用套餐，无法调用接口"
	case "rate limit exceeded":
		return "请求过于频繁，请稍后再试"
	case "public channel sold out":
		return "公共渠道额度已售罄，请选择其他套餐"
	case "protocol not supported by plan":
		return "当前套餐不支持该协议"
	case "protocol not supported by upstream":
		return "当前上游通道不支持该协议"
	case "no active upstream account bound":
		return "当前账号尚未绑定可用上游通道，请联系管理员开通"
	case "missing authorization token":
		return "缺少登录凭证，请重新登录"
	case "invalid authorization token":
		return "登录状态已失效，请重新登录"
	case "user not found":
		return "账号不存在，请重新登录"
	case "user disabled":
		return "账号已被禁用，请联系管理员"
	case "admin permission required":
		return "当前操作需要管理员权限"
	case "registration disabled":
		return "当前站点暂未开放新用户注册"
	case "upstream request failed":
		return "上游请求失败"
	}
	messages := map[string]string{
		"active subscription in effect":                 "当前套餐仍在有效期内，请待到期后再购买其他套餐",
		"account pending approval":                      "账号尚未通过审核，请等待管理员开通",
		"admin permission required":                     "当前操作需要管理员权限",
		"api key already exists":                        "每个账号仅允许保留一个 API Key，请使用“更新密钥”替换",
		"api key not found":                             "未找到 API Key",
		"api key secret unavailable":                    "该密钥无法解密展示，请使用“更新密钥”重新生成",
		"email already exists":                          "该邮箱已存在，请更换邮箱或直接登录",
		"email already registered":                      "该邮箱已注册，请直接登录",
		"email not verified":                            "邮箱尚未完成验证，请先通过邮箱验证后再登录",
		"failed to approve order":                       "订单审核失败，请稍后重试",
		"failed to create api key":                      "API Key 创建失败，请稍后重试",
		"failed to create captcha":                      "安全验证生成失败，请刷新后重试",
		"failed to create doc page":                     "文档创建失败，请检查填写内容",
		"failed to create email code":                   "邮箱验证码生成失败，请稍后重试",
		"failed to create order":                        "订单创建失败，请稍后重试",
		"failed to create plan":                         "套餐创建失败，请检查填写内容",
		"failed to decrypt api key":                     "密钥解密失败，请稍后重试或联系管理员",
		"failed to delete doc page":                     "文档删除失败，请稍后重试",
		"failed to delete plan":                         "套餐删除失败，请稍后重试",
		"failed to delete user":                         "用户删除失败，请稍后重试",
		"failed to disable api key":                     "API Key 禁用失败，请稍后重试",
		"failed to enable api key":                      "API Key 启用失败，请稍后重试",
		"failed to generate api key":                    "API Key 生成失败，请稍后重试",
		"failed to generate token":                      "登录凭证生成失败，请稍后重试",
		"failed to hash password":                       "密码处理失败，请稍后重试",
		"failed to list api keys":                       "读取 API Key 失败，请稍后重试",
		"failed to load doc page":                       "文档读取失败，请稍后重试",
		"failed to reject order":                        "订单拒绝失败，请稍后重试",
		"failed to rotate api key":                      "更新密钥失败，请稍后重试",
		"failed to save email code":                     "邮箱验证码保存失败，请稍后重试",
		"failed to update doc page":                     "文档更新失败，请检查填写内容",
		"failed to update order":                        "订单状态更新失败，请稍后重试",
		"failed to update password":                     "密码修改失败，请稍后重试",
		"failed to update plan":                         "套餐更新失败，请检查填写内容",
		"failed to update settings":                     "系统设置保存失败，请稍后重试",
		"failed to update user":                         "用户更新失败，请稍后重试",
		"free plan sold out":                            "免费套餐已领完",
		"free plan user limit reached":                  "你已达到该免费套餐的领取上限",
		"invalid api key":                               "API Key 无效，请检查后重试",
		"invalid authorization token":                   "登录状态已失效，请重新登录",
		"invalid credentials":                           "邮箱或密码不正确，请检查后重试",
		"invalid email verification code":               "邮箱验证码不正确或已过期",
		"invalid old password":                          "旧密码不正确，请重新输入",
		"invalid slide captcha":                         "安全验证未通过，请重新拖动滑块",
		"missing api key":                               "缺少 API Key",
		"missing authorization token":                   "缺少登录凭证，请重新登录",
		"no active subscription assigned":               "当前账号未分配有效套餐，已禁止调用，请联系管理员处理",
		"no active upstream account bound":              "当前账号尚未绑定可用上游通道，请联系管理员开通",
		"no api key to rotate":                          "当前没有可更新的 API Key，请先创建",
		"upstream rebinding required after plan change": "修改用户套餐后，必须重新绑定上游渠道并填写新的上游 API Key",
		"order already reviewed":                        "该订单已审核，请勿重复操作",
		"order already waiting review":                  "该套餐已有待审核订单，请勿重复提交",
		"order not found":                               "订单不存在或已被删除",
		"order not pending payment":                     "订单当前状态不允许继续支付，请刷新后查看",
		"password confirmation mismatch":                "两次输入的新密码不一致",
		"password must be at least 8 characters":        "密码至少需要 8 位",
		"payment config missing":                        "支付配置未完成，请联系管理员",
		"payment not completed":                         "支付结果尚未确认，请完成支付后再试",
		"plan not found":                                "套餐不存在或已下架",
		"plan price required":                           "套餐价格不能小于 0",
		"rate limit exceeded":                           "请求过于频繁，请稍后再试",
		"subscription expired":                          "订阅已到期，请续费后继续使用",
		"subscription quota exceeded":                   "本周美元额度已用完，请升级或续费后继续使用",
		"upstream channel is required":                  "必须绑定有效的上游渠道",
		"user disabled":                                 "账号已被禁用，请联系管理员",
		"user is not approved":                          "账号尚未通过审核，请等待管理员开通",
		"user not found":                                "账号不存在，请重新登录",
	}
	if localized, ok := messages[message]; ok {
		return localized
	}
	if strings.Contains(message, "Field validation") {
		return localizeValidationMessage(message)
	}
	if strings.HasPrefix(message, "failed to send email:") {
		return "邮件发送失败，请检查 SMTP 配置后重试"
	}
	return message
}

func localizeValidationMessage(message string) string {
	fieldLabels := map[string]string{
		"Username":    "用户名",
		"Email":       "邮箱",
		"Password":    "密码",
		"Name":        "名称",
		"BaseURL":     "API 地址",
		"APIURL":      "API 地址",
		"ModelName":   "模型名称",
		"Title":       "标题",
		"Slug":        "Slug",
		"Content":     "内容",
		"Subject":     "邮件标题",
		"Body":        "邮件内容",
		"NewPassword": "新密码",
	}
	if field, tag, param, ok := parseValidationError(message); ok {
		label := fieldLabels[field]
		if label == "" {
			label = field
		}
		switch tag {
		case "required":
			return fmt.Sprintf("请填写%s", label)
		case "email":
			return "请填写正确的邮箱地址"
		case "url":
			return fmt.Sprintf("请填写正确的%s", label)
		case "min":
			return fmt.Sprintf("%s至少需要 %s 位", label, param)
		case "max":
			return fmt.Sprintf("%s最多允许 %s 位", label, param)
		}
	}
	return "请检查表单必填项和格式是否正确"
}

func parseValidationError(message string) (string, string, string, bool) {
	field := between(message, "Field validation for '", "' failed")
	tag := between(message, "failed on the '", "' tag")
	param := ""
	if tag == "min" || tag == "max" {
		param = between(message, "Param: '", "'")
	}
	return field, tag, param, field != "" && tag != ""
}

func between(value, start, end string) string {
	startIndex := strings.Index(value, start)
	if startIndex < 0 {
		return ""
	}
	startIndex += len(start)
	endIndex := strings.Index(value[startIndex:], end)
	if endIndex < 0 {
		return ""
	}
	return value[startIndex : startIndex+endIndex]
}
