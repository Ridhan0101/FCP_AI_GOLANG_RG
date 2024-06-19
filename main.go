package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// AIModelConnector struct untuk menyimpan http.Client
type AIModelConnector struct {
	Client *http.Client
}

// Inputs struct untuk mendefinisikan format input untuk AI model
type Inputs struct {
	Table map[string][]string `json:"table"`
	Query string              `json:"query"`
}

// Response struct untuk mendefinisikan format response dari AI model
type Response struct {
	Answer      string   `json:"answer"`
	Coordinates [][]int  `json:"coordinates"`
	Cells       []string `json:"cells"`
	Aggregator  string   `json:"aggregator"`
}

// CsvToSlice fungsi untuk mengonversi CSV menjadi map
func CsvToSlice(data string) (map[string][]string, error) {
	reader := csv.NewReader(strings.NewReader(data))
	records, err := reader.ReadAll() // Baca semua data dari CSV
	if err != nil {
		return nil, err
	}

	if len(records) < 1 {
		return nil, errors.New("no data found")
	}

	header := records[0]
	result := make(map[string][]string)

	for i, col := range header {
		result[col] = make([]string, 0, len(records)-1)
		for _, record := range records[1:] {
			if i < len(record) {
				result[col] = append(result[col], record[i])
			}
		}
	}

	return result, nil
}

// ConnectAIModel fungsi untuk menghubungkan ke AI model dan mendapatkan response
func (c *AIModelConnector) ConnectAIModel(payload Inputs, token string) (Response, error) {
	url := "https://api-inference.huggingface.co/models/google/tapas-base-finetuned-wtq"
	data, err := json.Marshal(payload) // Konversi payload ke JSON
	if err != nil {
		return Response{}, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	// Retry logic untuk mencoba kembali koneksi ke model AI jika gagal
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		resp, err := c.Client.Do(req)
		if err != nil {
			return Response{}, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var aiResponse Response
			if err := json.NewDecoder(resp.Body).Decode(&aiResponse); err != nil {
				return Response{}, err
			}
			return aiResponse, nil
		}

		if resp.StatusCode == http.StatusServiceUnavailable {
			var result map[string]interface{}
			body, _ := ioutil.ReadAll(resp.Body)
			if err := json.Unmarshal(body, &result); err == nil {
				if estimatedTime, ok := result["estimated_time"].(float64); ok {
					log.Printf("Model is currently loading, retrying in %.1f seconds...\n", estimatedTime)
					time.Sleep(time.Duration(estimatedTime) * time.Second)
					continue
				}
			}
		}

		body, _ := ioutil.ReadAll(resp.Body)
		return Response{}, fmt.Errorf("failed to connect to AI model, status: %s, response: %s", resp.Status, string(body))
	}

	return Response{}, fmt.Errorf("max retries reached, failed to connect to AI model")
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v\n", err)
	}

	// Get Huggingface API token dari environment variables
	token := os.Getenv("HUGGINGFACE_TOKEN")
	if token == "" {
		log.Fatalf("HUGGINGFACE_TOKEN not found in .env file")
	}

	// Path to CSV file
	csvFile := "data-series.csv"

	// Baca CSV file
	data, err := ioutil.ReadFile(csvFile)
	if err != nil {
		log.Fatalf("Error reading CSV file: %v\n", err)
	}

	// Parse CSV to slice
	table, err := CsvToSlice(string(data))
	if err != nil {
		log.Fatalf("Error parsing CSV file: %v\n", err)
	}

	// Buat AI model connector
	client := &http.Client{}
	connector := &AIModelConnector{Client: client}

	// Mulai interaksi chatbot
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("AI-Powered Smart Home Energy Management System")
	fmt.Println("Enter your query (type 'exit' to quit):")

	for {
		fmt.Print("> ")
		scanner.Scan()
		query := scanner.Text()

		if strings.ToLower(query) == "exit" {
			break
		}

		payload := Inputs{
			Table: table,
			Query: query,
		}

		response, err := connector.ConnectAIModel(payload, token)
		if err != nil {
			log.Printf("Error connecting to AI model: %v\n", err)
			continue
		}

		// Tampilkan respons
		fmt.Println("Answer:", response.Answer)
		fmt.Println("Coordinates:", response.Coordinates)
		fmt.Println("Cells:", response.Cells)
		fmt.Println("Aggregator:", response.Aggregator)
		fmt.Println()
	}
}


