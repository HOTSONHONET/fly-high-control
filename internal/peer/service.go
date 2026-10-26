package peer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/HOTSONHONET/fly-high-control/internal/announce"
	"github.com/HOTSONHONET/fly-high-control/internal/config"
)

type AnnounceRequest struct {
	Message string `json:"message"`
	Repeat  int    `json:"repeat"`
}

type Result struct {
	Node  string
	Error error
}

type Service struct {
	cfg    config.Config
	runner *announce.Runner
	client *http.Client
}

func New(cfg config.Config, runner *announce.Runner) *Service {
	return &Service{
		cfg:    cfg,
		runner: runner,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/v1/announce", s.handleAnnounce)
	return mux
}

func (s *Service) handleAnnounce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("Authorization") != "Bearer "+s.cfg.ClusterToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req AnnounceRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Repeat < 1 || req.Repeat > announce.MaxRepeat || strings.TrimSpace(req.Message) == "" {
		http.Error(w, "invalid announcement", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := s.runner.RunLocal(ctx, req.Message, req.Repeat); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) Execute(ctx context.Context, cmd announce.Command) []Result {
	if cmd.Target == "all" {
		results := make([]Result, 0, 1+len(s.cfg.Peers))
		results = append(results, Result{Node: s.cfg.NodeName, Error: s.runner.RunLocal(ctx, cmd.Message, cmd.Repeat)})
		for name, p := range s.cfg.Peers {
			results = append(results, Result{Node: name, Error: s.send(ctx, p.URL, cmd.Message, cmd.Repeat)})
		}
		return results
	}

	if cmd.Target == s.cfg.NodeName {
		return []Result{{Node: s.cfg.NodeName, Error: s.runner.RunLocal(ctx, cmd.Message, cmd.Repeat)}}
	}

	p, ok := s.cfg.Peers[cmd.Target]
	if !ok {
		return []Result{{Node: cmd.Target, Error: fmt.Errorf("target not configured")}}
	}
	return []Result{{Node: cmd.Target, Error: s.send(ctx, p.URL, cmd.Message, cmd.Repeat)}}
}

func (s *Service) send(ctx context.Context, baseURL, message string, repeat int) error {
	body, err := json.Marshal(AnnounceRequest{Message: message, Repeat: repeat})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/announce", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.ClusterToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("peer returned %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}
