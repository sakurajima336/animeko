package api

import (
	"encoding/hex"
	"net/http"
	"strings"
)

// identity 一个客户端的身份。
type identity struct {
	id       string
	nickname string
	avatar   *string
}

// identityHints 客户端在请求体里显式透传的身份信息。
//
// 客户端把它所在官方房间里的成员信息(昵称、头像)带过来, 使聊天室显示的名字与头像
// 与官方「一起看」成员列表一致。
type identityHints struct {
	userID   string
	nickname string
	avatar   string
}

// identifyUser 识别请求用户。
//
// 自托管服务端不接入 Bangumi 账号体系,因此按以下优先级确定身份:
//  1. 请求体透传的 sender* 字段(官方房间成员信息),其次 query / header 里的同名字段;
//  2. Authorization: Bearer <token> —— 同一 token 视为同一用户(稳定且跨请求一致);
//  3. 兜底:按 IP + User-Agent 生成稳定匿名 ID。
//
// 生产部署若需要真实账号,可在此接入 Bangumi token 校验。
func identifyUser(r *http.Request, hints identityHints) identity {
	userID := firstNonEmpty(hints.userID, r.URL.Query().Get("userId"), r.Header.Get("X-Ani-User-Id"))
	nickname := firstNonEmpty(hints.nickname, r.URL.Query().Get("nickname"), r.Header.Get("X-Ani-Nickname"))
	avatar := optionalValue(hints.avatar, r.URL.Query().Get("avatar"), r.Header.Get("X-Ani-Avatar"))

	if userID != "" {
		if nickname == "" {
			nickname = "用户" + userID
		}
		return identity{id: userID, nickname: nickname, avatar: avatar}
	}

	if token := bearerToken(r); token != "" {
		h := hash(token)
		return identity{
			id:       "u_" + h,
			nickname: firstNonEmpty(nickname, "用户"+h[:6]),
			avatar:   avatar,
		}
	}

	anon := "anon_" + hash(clientIP(r)+"|"+r.UserAgent())
	return identity{
		id:       anon,
		nickname: firstNonEmpty(nickname, "游客"+anon[len(anon)-6:]),
		avatar:   avatar,
	}
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return strings.TrimSpace(strings.Split(v, ",")[0])
	}
	if v := r.Header.Get("X-Real-Ip"); v != "" {
		return v
	}
	if i := strings.LastIndex(r.RemoteAddr, ":"); i >= 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// optionalValue 返回第一个非空值; 全为空时返回 nil, 使 JSON 省略该字段。
func optionalValue(values ...string) *string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return &trimmed
		}
	}
	return nil
}

// hash 把凭据映射为稳定短摘要(FNV-1a,仅用于身份映射,非安全用途)。
func hash(s string) string {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return hex.EncodeToString([]byte{
		byte(h >> 56), byte(h >> 48), byte(h >> 40), byte(h >> 32),
		byte(h >> 24), byte(h >> 16), byte(h >> 8), byte(h),
	})
}
