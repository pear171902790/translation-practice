package main

import (
	"database/sql"
	"embed"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed templates/*.html
var templateFS embed.FS

const maxSentenceRunes = 4000

type pageData struct {
	Title     string
	Page      string
	Flash     string
	Error     string
	Chinese   string
	English   string
	Sentences []Sentence
	Count     int
	Sentence  *Sentence
	Attempt   string
	Revealed  bool
	Match     bool
}

// Server serves the add and practice pages.
type Server struct {
	store *Store
	tmpl  *template.Template
}

func NewServer(store *Store) (*Server, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{store: store, tmpl: tmpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /add", s.addForm)
	mux.HandleFunc("POST /add", s.addSave)
	mux.HandleFunc("GET /practice", s.practiceForm)
	mux.HandleFunc("POST /practice", s.practiceCheck)
	mux.HandleFunc("POST /sentences/{id}/delete", s.delete)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return logRequests(mux)
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.Count(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, "home", http.StatusOK, pageData{
		Title: "首页",
		Page:  "home",
		Count: count,
	})
}

func (s *Server) addForm(w http.ResponseWriter, r *http.Request) {
	data, err := s.addPage(r, pageData{Title: "添加句子", Page: "add"})
	if err != nil {
		s.fail(w, err)
		return
	}
	if r.URL.Query().Get("saved") == "1" {
		data.Flash = "已保存这句。"
	}
	s.render(w, "add", http.StatusOK, data)
}

func (s *Server) addSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "无法读取表单。", http.StatusBadRequest)
		return
	}
	chinese, english, fieldErr := cleanPair(r.FormValue("chinese"), r.FormValue("english"))
	if fieldErr != "" {
		data, err := s.addPage(r, pageData{
			Title:   "添加句子",
			Page:    "add",
			Chinese: strings.TrimSpace(r.FormValue("chinese")),
			English: strings.TrimSpace(r.FormValue("english")),
			Error:   fieldErr,
		})
		if err != nil {
			s.fail(w, err)
			return
		}
		s.render(w, "add", http.StatusBadRequest, data)
		return
	}
	if _, err := s.store.Create(r.Context(), chinese, english); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/add?saved=1", http.StatusSeeOther)
}

func (s *Server) addPage(r *http.Request, data pageData) (pageData, error) {
	sentences, err := s.store.List(r.Context())
	if err != nil {
		return data, err
	}
	data.Sentences = sentences
	data.Count = len(sentences)
	return data, nil
}

func (s *Server) practiceForm(w http.ResponseWriter, r *http.Request) {
	sentence, err := s.pickSentence(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, "practice", http.StatusOK, pageData{
		Title:    "练习",
		Page:     "practice",
		Sentence: sentence,
	})
}

func (s *Server) practiceCheck(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "无法读取表单。", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil || id <= 0 {
		s.render(w, "practice", http.StatusBadRequest, pageData{
			Title: "练习",
			Page:  "practice",
			Error: "找不到这句，可能已经删除了。",
		})
		return
	}
	sentence, err := s.store.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		s.render(w, "practice", http.StatusNotFound, pageData{
			Title: "练习",
			Page:  "practice",
			Error: "找不到这句，可能已经删除了。",
		})
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	attempt := strings.TrimSpace(r.FormValue("attempt"))
	data := pageData{
		Title:    "练习",
		Page:     "practice",
		Sentence: &sentence,
		Attempt:  attempt,
		Revealed: true,
	}
	if attempt != "" {
		data.Match = normalizeAnswer(attempt) == normalizeAnswer(sentence.English)
	}
	s.render(w, "practice", http.StatusOK, data)
}

func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "无效的句子。", http.StatusBadRequest)
		return
	}
	if err := s.store.Delete(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/add", http.StatusSeeOther)
}

func (s *Server) pickSentence(r *http.Request) (*Sentence, error) {
	if raw := r.URL.Query().Get("id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && id > 0 {
			sentence, err := s.store.Get(r.Context(), id)
			if err == nil {
				return &sentence, nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}
	}
	sentence, err := s.store.Random(r.Context())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sentence, nil
}

func (s *Server) render(w http.ResponseWriter, name string, status int, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	log.Printf("request error: %v", err)
	http.Error(w, "服务器出错了，请稍后再试。", http.StatusInternalServerError)
}

func cleanPair(chinese, english string) (string, string, string) {
	chinese = strings.TrimSpace(chinese)
	english = strings.TrimSpace(english)
	if chinese == "" || english == "" {
		return "", "", "请把中文和英文都填上。"
	}
	if utf8.RuneCountInString(chinese) > maxSentenceRunes || utf8.RuneCountInString(english) > maxSentenceRunes {
		return "", "", "这句话有点长，请控制在 4000 字以内。"
	}
	return chinese, english, ""
}

func normalizeAnswer(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d", r.Method, r.URL.RequestURI(), sw.code)
	})
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}
