package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

type FileStruct struct {
	S3ModificationTime time.Time
	FileSize           int64
	Filename           string
	FileTimestamp      time.Time
}

type (
	ByTimestamp          []FileStruct
	ByS3ModificationTime []FileStruct
)

func (f ByTimestamp) Len() int           { return len(f) }
func (f ByTimestamp) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f ByTimestamp) Less(i, j int) bool { return f[i].FileTimestamp.Before(f[j].FileTimestamp) }

func (f ByS3ModificationTime) Len() int      { return len(f) }
func (f ByS3ModificationTime) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f ByS3ModificationTime) Less(i, j int) bool {
	return f[i].S3ModificationTime.Before(f[j].S3ModificationTime)
}

func main() {
	filename := flag.String("file", "list.txt", "Path to the input file")
	sortBy := flag.String("sort", "timestamp", "Sort by 'timestamp' or 's3' modification time")
	sortOrder := flag.String("order", "asc", "Sort order: 'asc' or 'desc'")
	flag.Parse()

	file, err := os.Open(*filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var files []FileStruct

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		s3Timestamp, err := time.Parse("2006-01-02 15:04:05", fmt.Sprintf("%s %s", fields[0], fields[1]))
		if err != nil {
			log.Printf("Error parsing S3 modification timestamp for line '%s': %v", line, err)
			continue
		}

		fileSize, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			log.Printf("Error parsing file size for line '%s': %v", line, err)
			continue
		}
		filename := strings.Join(fields[3:], " ")

		fileTimestamp, err := extractFileTimestamp(filename, s3Timestamp)
		if err != nil {
			log.Printf("Error extracting file timestamp for line '%s': %v", line, err)
			continue
		}
		files = append(files, FileStruct{
			S3ModificationTime: s3Timestamp,
			FileSize:           fileSize,
			Filename:           filename,
			FileTimestamp:      fileTimestamp,
		})
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// Sort the files based on the specified flag
	switch *sortBy {
	case "timestamp":
		if *sortOrder == "desc" {
			sort.Sort(sort.Reverse(ByTimestamp(files)))
		} else {
			sort.Sort(ByTimestamp(files))
		}
	case "s3":
		if *sortOrder == "desc" {
			sort.Sort(sort.Reverse(ByS3ModificationTime(files)))
		} else {
			sort.Sort(ByS3ModificationTime(files))
		}
	default:
		log.Fatal("Invalid sort option. Use 'timestamp' or 's3'.")
	}

	filePaths := []string{
		"rm.sh",
		"sync.sh",
	}

	for _, filePath := range filePaths {
		// Check if the file exists
		if _, err := os.Stat(filePath); err == nil {
			// File exists, so delete it
			err := os.Remove(filePath)
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Deleted file:", filePath)
		} else if os.IsNotExist(err) {
			// File does not exist
			log.Println("File does not exist:", filePath)
		} else {
			// Error occurred while checking file existence
			log.Fatal(err)
		}
	}

	// Print the sorted files with relative timestamps
	fmt.Println("Sorted Files:")
	for _, file := range files {
		relativeTime := formatRelativeTime(file.FileTimestamp)
		fmt.Printf("S3 Modification Time: %s, %s, %s, age: %s\n",
			file.S3ModificationTime.Format("2006-01-02 15:04:05"), humanize.Bytes(uint64(file.FileSize)), file.Filename, relativeTime)

		// Write the command to stdout with proper quoting in bash
		comment := fmt.Sprintf("# S3 Modification Time: %s, %s, %s, age: %s\n",
			file.S3ModificationTime.Format("2006-01-02 15:04:05"), humanize.Bytes(uint64(file.FileSize)), file.Filename, relativeTime)
		rmCommand := fmt.Sprintf("aws s3 rm 's3://streamboxdineorb/%s'\n", strings.ReplaceAll(file.Filename, "'", "'\"'\"'"))
		writeToFile("rm.sh", comment+rmCommand)

		// Write the sync command to sync.sh with a comment
		comment = fmt.Sprintf("# S3 Modification Time: %s, %s, %s, age: %s\n",
			file.S3ModificationTime.Format("2006-01-02 15:04:05"), humanize.Bytes(uint64(file.FileSize)), file.Filename, relativeTime)
		syncCommand := fmt.Sprintf("aws s3 sync 's3://streamboxdineorb' /tmp/video --exclude='*' --include='%s'\n", file.Filename)
		writeToFile("sync.sh", comment+syncCommand)
	}

	// Marshal the sorted files to JSON with indented formatting
	jsonData, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal to JSON:", err)
	}

	// Write the JSON data to a file
	outputFile, err := os.Create("results.json")
	if err != nil {
		log.Fatal("Failed to create output file:", err)
	}
	defer outputFile.Close()

	_, err = outputFile.Write(jsonData)
	if err != nil {
		log.Fatal("Failed to write JSON data to file:", err)
	}

	fmt.Println("Results saved to results.json")
}

func extractFileTimestamp(filename string, s3Timestamp time.Time) (time.Time, error) {
	// Define a regular expression pattern to match the timestamp in the filename
	pattern := `(\d{8}_\d{6})`

	// Compile the regular expression
	regex := regexp.MustCompile(pattern)

	// Find the timestamp in the filename
	match := regex.FindStringSubmatch(filename)
	if match != nil {
		// Extract the timestamp substring from the match
		timestampStr := match[0]

		// Parse the timestamp
		fileTimestamp, err := time.Parse("20060102_150405", timestampStr)
		if err != nil {
			return s3Timestamp, fmt.Errorf("unable to parse file timestamp: %v", err)
		}

		return fileTimestamp, nil
	}

	// Return the S3 timestamp if the file timestamp is not found in the filename
	return s3Timestamp, nil
}

func formatRelativeTime(timestamp time.Time) string {
	duration := time.Since(timestamp)
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	var relativeTime string
	if days > 0 {
		relativeTime += fmt.Sprintf("%dd ", days)
	}
	if hours > 0 {
		relativeTime += fmt.Sprintf("%dh ", hours)
	}
	if minutes > 0 {
		relativeTime += fmt.Sprintf("%dm ", minutes)
	}
	if seconds > 0 {
		relativeTime += fmt.Sprintf("%ds", seconds)
	}

	return relativeTime
}

func writeToFile(filename, content string) {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("Failed to open file '%s': %v", filename, err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		log.Fatalf("Failed to write to file '%s': %v", filename, err)
	}
}
