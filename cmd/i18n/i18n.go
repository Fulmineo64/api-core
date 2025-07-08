package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Placeholder struct {
	ID             string `json:"id"`
	String         string `json:"string"`
	Type           string `json:"type"`
	UnderlyingType string `json:"underlyingType"`
	ArgNum         int    `json:"argNum"`
	Expr           string `json:"expr"`
}

type Message struct {
	ID           string        `json:"id"`
	Message      string        `json:"message"`
	Translation  string        `json:"translation"`
	Placeholders []Placeholder `json:"placeholders,omitempty"`
}

type MessageFile struct {
	Language string    `json:"language"`
	Messages []Message `json:"messages"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run i18n.go <workingDir> [gotext args...]")
		os.Exit(1)
	}

	fmt.Println("🌍 Generazione traduzioni...")

	godotenv.Load()

	workingDir := os.Args[1]
	gotextArgs := os.Args[2:]

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	apiModel := os.Getenv("OPENROUTER_API_MODEL")

	runGoText(workingDir, gotextArgs...)

	if apiKey == "" || apiModel == "" {
		fmt.Println("Imposta le variabili d'ambiente OPENROUTER_API_KEY e OPENROUTER_API_MODEL (https://openrouter.ai) per tradurre automaticamente le nuove aggiunte.")
		return
	}

	err := filepath.Walk(workingDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "out.gotext.json" {
			processFile(path, apiKey, apiModel)
		}
		return nil
	})
	if err != nil {
		fmt.Println("Errore nella scansione dei file:", err)
		os.Exit(1)
	}

	runGoText(workingDir, gotextArgs...)

	fmt.Println("✅ Generazione traduzioni completata.")
}

func runGoText(dir string, args ...string) {
	cmd := exec.Command("gotext", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("Errore durante l'esecuzione di gotext:", err)
		os.Exit(1)
	}
}

func processFile(path, apiKey, apiModel string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Errore lettura file:", err)
		return
	}

	var msgFile MessageFile
	err = json.Unmarshal(data, &msgFile)
	if err != nil {
		fmt.Println("Errore parsing JSON:", err)
		return
	}

	var toTranslate []Message
	for _, m := range msgFile.Messages {
		if strings.TrimSpace(m.Translation) == "" {
			toTranslate = append(toTranslate, m)
		}
	}

	if len(toTranslate) == 0 {
		return
	}

	fmt.Println("✨ Traduzione automatica nuove aggiunte in ", path, "...")

	translated, err := translateBatch(apiKey, apiModel, toTranslate, msgFile.Language)
	if err != nil {
		fmt.Println("Errore traduzione batch:", err)
		return
	}

	for i, original := range msgFile.Messages {
		for _, t := range translated {
			if t.ID == original.ID {
				msgFile.Messages[i].Translation = t.Translation
			}
		}
	}

	output, _ := json.MarshalIndent(msgFile, "", "  ")
	newPath := filepath.Join(filepath.Dir(path), "messages.gotext.json")
	err = os.WriteFile(newPath, output, 0644)
	if err != nil {
		fmt.Println("Errore salvataggio file:", err)
	}
}

func translateBatch(apiKey, apiModel string, messages []Message, lang string) ([]Message, error) {
	prompt := fmt.Sprintf(
		"Traduci i seguenti messaggi nella lingua \"%s\". Ogni oggetto ha un campo \"message\" da tradurre. Mantieni tutti gli altri campi inalterati, e restituisci un array identico con la proprietà \"translation\" valorizzata.",
		lang,
	)

	inputJSON, _ := json.Marshal(messages)
	reqBody := map[string]interface{}{
		"model": apiModel,
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
			{"role": "user", "content": string(inputJSON)},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "Go Translation Tool")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var possibleError struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bodyBytes, &possibleError); err == nil && possibleError.Error.Message != "" {
		fmt.Fprintf(os.Stderr, "Errore da OpenRouter (%d): %s\n", possibleError.Error.Code, possibleError.Error.Message)
		os.Exit(1)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("nessuna scelta nella risposta")
	}

	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	content := message["content"].(string)

	content = strings.TrimSpace(content)
	content = strings.Trim(content, "```json")
	content = strings.Trim(content, "```")

	var translated []Message
	if err := json.Unmarshal([]byte(content), &translated); err != nil {
		return nil, fmt.Errorf("errore nel parsing JSON tradotto: %w\nContenuto: %s", err, content)
	}

	return translated, nil
}
