package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/cors"
	"github.com/tealeg/xlsx"
)

type DocMakerProps struct {
	Data      []map[string]interface{} `json:"data"`
	TypeofDoc string                   `json:"typeofDoc"`
}

func generateCSV(data []map[string]interface{}) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data provided")
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	headers := make([]string, 0)
	for key := range data[0] {
		headers = append(headers, key)
	}
	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	for _, record := range data {
		recordValues := make([]string, len(headers))
		for i, header := range headers {
			if value, ok := record[header]; ok {
				recordValues[i] = fmt.Sprintf("%v", value)
			}
		}
		if err := writer.Write(recordValues); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func generateXLSX(data []map[string]interface{}) ([]byte, error) {
	file := xlsx.NewFile()
	sheet, err := file.AddSheet("Sheet1")
	if err != nil {
		return nil, err
	}

	if len(data) > 0 {
		headerRow := sheet.AddRow()
		for key := range data[0] {
			cell := headerRow.AddCell()
			cell.Value = key
		}

		for _, record := range data {
			row := sheet.AddRow()
			for _, key := range getKeys(record) {
				cell := row.AddCell()
				cell.Value = fmt.Sprintf("%v", record[key])
			}
		}
	}

	var buf bytes.Buffer
	err = file.Write(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func makeFile(data []byte, docType string) (string, error) {
	fileName := fmt.Sprintf("file_%d.%s", time.Now().Unix(), docType)
	filePath := filepath.Join("/tmp", fileName)

	err := os.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", err
	}
	return filePath, nil
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		var props DocMakerProps
		if err := json.NewDecoder(r.Body).Decode(&props); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var fileBuffer []byte
		var err error

		if props.TypeofDoc == "csv" {
			fileBuffer, err = generateCSV(props.Data)
		} else if props.TypeofDoc == "xlsx" {
			fileBuffer, err = generateXLSX(props.Data)
		} else {
			http.Error(w, "Invalid document type", http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("Error generating file: %v", err), http.StatusInternalServerError)
			return
		}

		filePath, err := makeFile(fileBuffer, props.TypeofDoc)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error saving file: %v", err), http.StatusInternalServerError)
			return
		}

		fileName := filepath.Base(filePath)
		downloadLink := fmt.Sprintf("http://localhost:3000/download?file=%s", fileName)
		response := map[string]string{"downloadLink": downloadLink}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	case "GET":
		fileName := r.URL.Query().Get("file")
		if fileName == "" {
			http.Error(w, "File name is missing", http.StatusBadRequest)
			return
		}

		filePath := filepath.Join("/tmp", fileName)
		fileBytes, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		w.Write(fileBytes)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRequest)

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	})

	handler := c.Handler(mux)
	log.Println("Server listening on port 3000")
	log.Fatal(http.ListenAndServe(":3000", handler))
}
