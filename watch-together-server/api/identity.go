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

// identifyUser 识别请求用户。
//
// 自托管服务端不接入 Bangumi 账号体系,因此按以下优先级确定身份:
//  1. 显式传入的 userId / nickname(query 或 header),便于调试与第三方集成;
//  2. Authorization: Bearer <token> —— 同一 token 视为同一用户(稳定且跨请求一致);
//  3. 兜底:按 IP + User-Agent 生成稳定匿名 ID。
//
// 生产部署若需要真实账号,可在此接入 Bangumi token 校验。
func identifyUser(r *http.Request, _ string) identity {
	if uid := firstNonEmpty(r.URL.Query().Get("userId"), r.Header.Get("X-Ani-User-Id")); uid != "" {
		nick := firstNonEmpty(r.URL.Query().Get("nickname"), r.Header.Get("X-Ani-Nickname"))
		if nick == "" {
			nick = "用户" + uid
		}
		return identity{id: uid, nickname: nick}
	}

	if token := bearerToken(r); token != "" {
		h := hash(token)
		return identity{
			id:       "u_" + h,
			nickname: "用户" + h[:6],
		}
	}

	anon := "anon_" + hash(clientIP(r)+"|"+r.UserAgent())
	return identity{id: anon, nickname: "游客" + anon[len(anon)-6:]}
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
