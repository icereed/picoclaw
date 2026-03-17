package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

// Ensure OpenAITranscriber satisfies the Transcriber interface at compile time.
var _ Transcriber = (*OpenAITranscriber)(nil)

func TestGroqTranscriberName(t *testing.T) {
	tr := NewGroqTranscriber("sk-test")
	if got := tr.Name(); got != "groq" {
		t.Errorf("Name() = %q, want %q", got, "groq")
	}
}

func TestOpenAITranscriberName(t *testing.T) {
	tr := NewOpenAITranscriber("my-provider", "sk-test", "https://example.com/v1", "whisper-1")
	if got := tr.Name(); got != "my-provider" {
		t.Errorf("Name() = %q, want %q", got, "my-provider")
	}
}

func TestDetectTranscriber(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantNil  bool
		wantName string
	}{
		{
			name:    "no config",
			cfg:     &config.Config{},
			wantNil: true,
		},
		{
			name: "groq provider key",
			cfg: &config.Config{
				Providers: config.ProvidersConfig{
					Groq: config.ProviderConfig{APIKey: "sk-groq-direct"},
				},
			},
			wantName: "groq",
		},
		{
			name: "groq via model list",
			cfg: &config.Config{
				ModelList: []config.ModelConfig{
					{Model: "openai/gpt-4o", APIKey: "sk-openai"},
					{Model: "groq/llama-3.3-70b", APIKey: "sk-groq-model"},
				},
			},
			wantName: "groq",
		},
		{
			name: "groq model list entry without key is skipped",
			cfg: &config.Config{
				ModelList: []config.ModelConfig{
					{Model: "groq/llama-3.3-70b", APIKey: ""},
				},
			},
			wantNil: true,
		},
		{
			name: "provider key takes priority over model list",
			cfg: &config.Config{
				Providers: config.ProvidersConfig{
					Groq: config.ProviderConfig{APIKey: "sk-groq-direct"},
				},
				ModelList: []config.ModelConfig{
					{Model: "groq/llama-3.3-70b", APIKey: "sk-groq-model"},
				},
			},
			wantName: "groq",
		},
		{
			name: "voice transcription config takes highest priority",
			cfg: &config.Config{
				Voice: config.VoiceConfig{
					Transcription: config.TranscriptionConfig{
						APIBase: "https://api.openai.com/v1",
						APIKey:  "sk-openai",
						Model:   "whisper-1",
					},
				},
				Providers: config.ProvidersConfig{
					Groq: config.ProviderConfig{APIKey: "sk-groq-direct"},
				},
			},
			wantName: "openai-compat",
		},
		{
			name: "voice transcription config defaults model to whisper-1",
			cfg: &config.Config{
				Voice: config.VoiceConfig{
					Transcription: config.TranscriptionConfig{
						APIBase: "https://api.example.com/v1",
						APIKey:  "sk-test",
					},
				},
			},
			wantName: "openai-compat",
		},
		{
			name: "voice transcription config incomplete (missing key)",
			cfg: &config.Config{
				Voice: config.VoiceConfig{
					Transcription: config.TranscriptionConfig{
						APIBase: "https://api.example.com/v1",
					},
				},
			},
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := DetectTranscriber(tc.cfg)
			if tc.wantNil {
				if tr != nil {
					t.Errorf("DetectTranscriber() = %v, want nil", tr)
				}
				return
			}
			if tr == nil {
				t.Fatal("DetectTranscriber() = nil, want non-nil")
			}
			if got := tr.Name(); got != tc.wantName {
				t.Errorf("Name() = %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestTranscribe(t *testing.T) {
	// Write a minimal fake audio file so the transcriber can open and send it.
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "clip.ogg")
	if err := os.WriteFile(audioPath, []byte("fake-audio-data"), 0o644); err != nil {
		t.Fatalf("failed to write fake audio file: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/audio/transcriptions" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer sk-test" {
				t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
			}
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Fatalf("failed to parse multipart form: %v", err)
			}
			if got := r.FormValue("model"); got != "whisper-large-v3" {
				t.Errorf("model = %q, want %q", got, "whisper-large-v3")
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(TranscriptionResponse{
				Text:     "hello world",
				Language: "en",
				Duration: 1.5,
			})
		}))
		defer srv.Close()

		tr := NewGroqTranscriber("sk-test")
		tr.apiBase = srv.URL

		resp, err := tr.Transcribe(context.Background(), audioPath)
		if err != nil {
			t.Fatalf("Transcribe() error: %v", err)
		}
		if resp.Text != "hello world" {
			t.Errorf("Text = %q, want %q", resp.Text, "hello world")
		}
		if resp.Language != "en" {
			t.Errorf("Language = %q, want %q", resp.Language, "en")
		}
	})

	t.Run("api error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"invalid_api_key"}`, http.StatusUnauthorized)
		}))
		defer srv.Close()

		tr := NewGroqTranscriber("sk-bad")
		tr.apiBase = srv.URL

		_, err := tr.Transcribe(context.Background(), audioPath)
		if err == nil {
			t.Fatal("expected error for non-200 response, got nil")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		tr := NewGroqTranscriber("sk-test")
		_, err := tr.Transcribe(context.Background(), filepath.Join(tmpDir, "nonexistent.ogg"))
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})

	t.Run("custom model", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Fatalf("failed to parse multipart form: %v", err)
			}
			if got := r.FormValue("model"); got != "whisper-1" {
				t.Errorf("model = %q, want %q", got, "whisper-1")
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(TranscriptionResponse{Text: "custom model result"})
		}))
		defer srv.Close()

		tr := NewOpenAITranscriber("test", "sk-test", srv.URL, "whisper-1")
		resp, err := tr.Transcribe(context.Background(), audioPath)
		if err != nil {
			t.Fatalf("Transcribe() error: %v", err)
		}
		if resp.Text != "custom model result" {
			t.Errorf("Text = %q, want %q", resp.Text, "custom model result")
		}
	})
}
