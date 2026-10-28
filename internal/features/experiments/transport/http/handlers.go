package http

import (
	"net/http"

	"beeline-games/internal/core/transport/http/request"
	"beeline-games/internal/core/transport/http/response"
)

// Create запускает сборку вариантов в фоне (202); ход сборки — через GET /api/experiments/{id}.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := request.RequireUserID(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var in createRequest
	if err := request.Decode(r, &in); err != nil {
		response.Error(w, r, err)
		return
	}

	x, variants, err := h.svc.Create(r.Context(), userID, in.Prompt, in.Variants)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusAccepted, h.toViewDTO(x, variants))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := request.RequireUserID(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	list, err := h.svc.List(r.Context(), userID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	out := make([]experimentDTO, 0, len(list))
	for _, x := range list {
		out = append(out, h.toExperimentDTO(x))
	}
	response.JSON(w, http.StatusOK, out)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, id, err := callerAndExperiment(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	x, variants, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toViewDTO(x, variants))
}

func (h *Handler) Launch(w http.ResponseWriter, r *http.Request) {
	userID, id, err := callerAndExperiment(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	x, variants, err := h.svc.Launch(r.Context(), userID, id)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toViewDTO(x, variants))
}

func (h *Handler) ChooseWinner(w http.ResponseWriter, r *http.Request) {
	userID, id, err := callerAndExperiment(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var in winnerRequest
	if err := request.Decode(r, &in); err != nil {
		response.Error(w, r, err)
		return
	}

	x, variants, err := h.svc.ChooseWinner(r.Context(), userID, id, in.GameID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toViewDTO(x, variants))
}

func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	userID, id, err := callerAndExperiment(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	rep, err := h.svc.Report(r.Context(), userID, id)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, h.toReportDTO(rep))
}

func callerAndExperiment(r *http.Request) (userID, id string, err error) {
	if userID, err = request.RequireUserID(r); err != nil {
		return "", "", err
	}
	if id, err = request.PathID(r, "id"); err != nil {
		return "", "", err
	}
	return userID, id, nil
}
