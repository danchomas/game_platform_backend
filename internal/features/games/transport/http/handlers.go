package http

import (
	"net/http"

	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

func (h *Handler) GetGame(w http.ResponseWriter, r *http.Request) {
	gameID, err := request.PathID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	userID, _ := request.UserID(r)

	g, err := h.svc.GetGame(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toGameDTO(g))
}

func (h *Handler) MyGames(w http.ResponseWriter, r *http.Request) {
	userID, err := request.RequireUserID(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	gs, err := h.svc.GetMyGames(r.Context(), userID, request.Int(r, "limit", 0), request.Int(r, "offset", 0))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toGameDTOs(gs))
}

func (h *Handler) Gallery(w http.ResponseWriter, r *http.Request) {
	gs, err := h.svc.GetGallery(r.Context(),
		request.Str(r, "sort"), request.Str(r, "q"),
		request.Int(r, "limit", 0), request.Int(r, "offset", 0))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toGameDTOs(gs))
}

func (h *Handler) PatchGame(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var in patchRequest
	if err := request.Decode(r, &in); err != nil {
		response.Error(w, r, err)
		return
	}

	g, err := h.svc.PatchGame(r.Context(), userID, gameID, in.Title)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toGameDTO(g))
}

func (h *Handler) DeleteGame(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.svc.DeleteGame(r.Context(), userID, gameID); err != nil {
		response.Error(w, r, err)
		return
	}

	response.NoContent(w)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	h.setPublished(w, r, true)
}

func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) {
	h.setPublished(w, r, false)
}

func (h *Handler) setPublished(w http.ResponseWriter, r *http.Request, publish bool) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	g, err := h.svc.PublishGame(r.Context(), userID, gameID, publish)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toGameDTO(g))
}

func (h *Handler) Messages(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	msgs, err := h.svc.GetMessages(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, toMessageDTOs(msgs))
}

func (h *Handler) Versions(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	vs, err := h.svc.GetVersions(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, toVersionDTOs(vs))
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	userID, gameID, err := h.callerAndGame(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	version, err := request.PathInt(r, "v")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	game, reply, newVersion, err := h.svc.RestoreVersion(r.Context(), userID, gameID, version)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toGenerationDTO(game, reply, newVersion))
}
