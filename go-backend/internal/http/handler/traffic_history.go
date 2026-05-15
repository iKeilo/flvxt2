package handler

import (
	"net/http"

	"go-backend/internal/auth"
	"go-backend/internal/http/middleware"
	"go-backend/internal/http/response"
)

func (h *Handler) trafficHistoryList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("请求失败"))
		return
	}

	claims, ok := r.Context().Value(middleware.ClaimsContextKey).(auth.Claims)
	if !ok {
		response.WriteJSON(w, response.Err(401, "无效的token或token已过期"))
		return
	}

	currentUserID, err := parseUserID(claims.Sub)
	if err != nil {
		response.WriteJSON(w, response.Err(401, "无效的token或token已过期"))
		return
	}

	var req struct {
		UserID int64 `json:"userId"`
		Limit  int   `json:"limit"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("请求参数错误"))
		return
	}
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 200 {
		req.Limit = 200
	}

	isAdmin := claims.RoleID == 0

	if !isAdmin || req.UserID <= 0 {
		items, err := h.repo.GetUserTrafficHistories(currentUserID, req.Limit)
		if err != nil {
			response.WriteJSON(w, response.Err(-2, err.Error()))
			return
		}
		response.WriteJSON(w, response.OK(items))
		return
	}

	items, err := h.repo.GetAllUserTrafficHistories(req.Limit)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	response.WriteJSON(w, response.OK(items))
}

func (h *Handler) trafficHistoryDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("请求失败"))
		return
	}

	claims, ok := r.Context().Value(middleware.ClaimsContextKey).(auth.Claims)
	if !ok {
		response.WriteJSON(w, response.Err(401, "无效的token或token已过期"))
		return
	}

	currentUserID, err := parseUserID(claims.Sub)
	if err != nil {
		response.WriteJSON(w, response.Err(401, "无效的token或token已过期"))
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("请求参数错误"))
		return
	}
	if req.ID <= 0 {
		response.WriteJSON(w, response.ErrDefault("历史记录 ID 不能为空"))
		return
	}

	isAdmin := claims.RoleID == 0
	if !isAdmin {
		histories, err := h.repo.GetUserTrafficHistories(currentUserID, 200)
		if err != nil {
			response.WriteJSON(w, response.Err(-2, err.Error()))
			return
		}
		found := false
		for _, item := range histories {
			if item.ID == req.ID {
				found = true
				break
			}
		}
		if !found {
			response.WriteJSON(w, response.Err(403, "无权删除此记录"))
			return
		}
	}

	if err := h.repo.DeleteUserTrafficHistory(req.ID); err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}

	response.WriteJSON(w, response.OKEmpty())
}
