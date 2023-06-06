package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FileStruct struct {
	S3ModificationTime time.Time
	FileSize           int64
	Filename           string
	FileTimestamp      time.Time
}

type ByTimestamp []FileStruct
type ByS3ModificationTime []FileStruct

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

		timestamp, err := time.Parse("2006-01-02 15:04:05", fmt.Sprintf("%s %s", fields[0], fields[1]))
		if err != nil {
			log.Printf("Error parsing timestamp for line '%s': %v", line, err)
			continue
		}

		fileSize, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			log.Printf("Error parsing file size for line '%s': %v", line, err)
			continue
		}

		files = append(files, FileStruct{
			S3ModificationTime: timestamp,
			FileSize:           fileSize,
			Filename:           fields[3],
			FileTimestamp:      timestamp,
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

	// Print the sorted files with relative timestamps
	fmt.Println("Sorted Files:")
	for _, file := range files {
		relativeTime := time.Since(file.FileTimestamp)
		fmt.Printf("S3 Modification Time: %s, File Size: %d bytes, Filename: %s, Relative Timestamp: %s\n",
			file.S3ModificationTime.Format("2006-01-02 15:04:05"), file.FileSize, file.Filename, formatRelativeTime(relativeTime))
	}
}

// Formats the given duration to a human-readable relative time format
func formatRelativeTime(duration time.Duration) string {
	seconds := int64(duration.Seconds())

	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm", seconds/60)
	case seconds < 86400:
		return fmt.Sprintf("%dh", seconds/3600)
	case seconds < 604800:
		return fmt.Sprintf("%dd", seconds/86400)
	default:
		return fmt.Sprintf("%dy", seconds/31536000)
	}
}
