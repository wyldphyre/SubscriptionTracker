package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/craigr/subscriptiontracker/internal/store"
)

// TagRowViewModel holds data for a single tag row in the management modal.
type TagRowViewModel struct {
	Tag        string
	UsageCount int
	Error      string
}

// TagsModalViewModel is passed to the tags management modal template.
type TagsModalViewModel struct {
	Tags []TagRowViewModel
}

// TagsModal handles GET /tags — opens the tag management modal.
func (h *Handlers) TagsModal(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "tags_modal.html", h.buildTagsModalVM())
}

// RenameTag handles PUT /tags/{tag}
func (h *Handlers) RenameTag(w http.ResponseWriter, r *http.Request) {
	oldName := r.PathValue("tag")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	newName := strings.ToLower(strings.TrimSpace(r.FormValue("name")))

	// Build the usage map once and reuse it for every response branch.
	usageMap := h.buildUsageMap()

	if newName == "" {
		vm := buildTagRow(oldName, usageMap[oldName], "Tag name cannot be empty")
		w.WriteHeader(http.StatusUnprocessableEntity)
		h.render(w, r, "tag_row.html", vm)
		return
	}

	if newName == oldName {
		h.render(w, r, "tag_row.html", buildTagRow(oldName, usageMap[oldName], ""))
		return
	}

	// Check for collision with existing tag
	for _, t := range h.store.ListTags() {
		if t == newName {
			vm := buildTagRow(oldName, usageMap[oldName], "A tag named \""+newName+"\" already exists")
			w.WriteHeader(http.StatusUnprocessableEntity)
			h.render(w, r, "tag_row.html", vm)
			return
		}
	}

	if err := h.store.RenameTag(oldName, newName); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Usage count carries over to the new name unchanged.
	w.Header().Set("HX-Trigger", `{"showToast":"Tag renamed"}`)
	h.render(w, r, "tag_row.html", buildTagRow(newName, usageMap[oldName], ""))
}

// DeleteTag handles DELETE /tags/{tag}
func (h *Handlers) DeleteTag(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("tag")
	if err := h.store.DeleteTag(name); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", `{"showToast":"Tag deleted"}`)
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) buildTagsModalVM() TagsModalViewModel {
	tags := h.store.ListTags()
	usageMap := h.buildUsageMap()
	rows := make([]TagRowViewModel, len(tags))
	for i, t := range tags {
		rows[i] = TagRowViewModel{Tag: t, UsageCount: usageMap[t]}
	}
	return TagsModalViewModel{Tags: rows}
}

func (h *Handlers) buildUsageMap() map[string]int {
	m := map[string]int{}
	for _, sub := range h.store.GetAll() {
		for _, t := range sub.Tags {
			m[t]++
		}
	}
	return m
}

func buildTagRow(tag string, usageCount int, errMsg string) TagRowViewModel {
	return TagRowViewModel{Tag: tag, UsageCount: usageCount, Error: errMsg}
}
