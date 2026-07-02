package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
)

func main() {
	var targetCourses []string
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Enter your course codes (press Enter on a blank line to start):")
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())

		// If the user hits enter without typing anything, break the loop
		if input == "" {
			break
		}

		// Add the course to our list
		targetCourses = append(targetCourses, input)
		fmt.Printf("Your courses: %v\n", targetCourses)
	}
	type exam struct {
		courseCode  string
		courseTitle string
		examDate    string
		examDay     string
		examTime    string
		examPlace   string
	}
	rawText, err := readPdf("Exams.pdf")

	if err != nil {
		log.Fatalf("Failed to read %v", err)
	}

	// we filtering the raw

	words := strings.Fields(rawText)

	// Loop using an index so we can check neighboring words
	for i := 0; i < len(words); i++ {
		// 1. Check if words[i] matches any course code in out list
		if isTargetCourse(words[i], targetCourses) {
			fmt.Printf("Found target course! %s\n", words[i])

			// 2. Now we scan forward from here to find the end of this exam section
			for j := i + 1; j < len(words); j++ {
				if words[j] == "College" || words[j] == "Online" {
					// SAFETY CHECK: Ensure we have enough words before 'j'
					// to prevent an "index out of range" panic.
					if j-6 < i+1 {
						continue
					}
					currentExam := exam{
						courseCode: words[i],
						// Join all words between the code and the date to form the title
						courseTitle: strings.Join(words[i+1:j-6], " "),
						examDate:    words[j-6],
						examDay:     words[j-5],
						// Combine the time pieces (e.g., "09:00", "-11:00", "AM")
						examTime:  strings.Join(words[j-4:j-1], " "),
						examPlace: words[j],
					}

					// 2. Display the formatted schedule to the user
					fmt.Println("\n------")
					fmt.Printf("Code:  %s\n", currentExam.courseCode)
					fmt.Printf("Title: %s\n", currentExam.courseTitle)
					fmt.Printf("Date:  %s (%s)\n", currentExam.examDate, currentExam.examDay)
					fmt.Printf("Time:  %s\n", currentExam.examTime)
					fmt.Printf("Place: %s\n", currentExam.examPlace)

					// Move our outer loop index forward so we don't re-parse inside this block

					i = j
					break
				}
			}
		}
	}

}

func isTargetCourse(word string, targets []string) bool {
	for _, target := range targets {
		if strings.EqualFold(word, target) {
			return true
		}
	}
	return false
}

func readPdf(path string) (string, error) {
	// Open the local PDF file
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Use a bytes.Buffer to capture text fragments efficiently
	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}

	_, err = buf.ReadFrom(b)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
