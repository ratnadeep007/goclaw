package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"ratnadeep007/goclaw/internal/agent"
	"ratnadeep007/goclaw/internal/llm"
	"ratnadeep007/goclaw/internal/session"
)

type Server struct {
	Sess *session.Session
	mux  *http.ServeMux
}

type TaskRequest struct {
	Task string `json:"task"`
}

type TaskResponse struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func NewSession(sess *session.Session) *Server {
	mux := http.NewServeMux()
	s := &Server{Sess: sess, mux: mux}
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/task", s.task)
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) task(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	history := append([]llm.Message(nil), s.Sess.Messages...)
	_ = s.Sess.AppendMessage("user", req.Task)
	res, _, err := agent.RunTask(s.Sess.Config, s.Sess.Memory, req.Task, history, s.Sess.SyncMemory, nil)
	if err == nil {
		_ = s.Sess.AppendMessage("assistant", res)
		_ = s.Sess.SyncMemory()
	}
	resp := TaskResponse{Result: res}
	if err != nil {
		resp.Error = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) Serve(ctx context.Context, addr string) error {
	server := &http.Server{Addr: addr, Handler: s.Handler()}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
