package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/ledongthuc/pdf"
)

type exam struct {
	courseCode  string
	courseTitle string
	examDate    string
	examDay     string
	examTime    string
	examPlace   string
}

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

	var outputMode int
	fmt.Println("\nChoose 1 for terminal version and 2 for ics version")
	_, err := fmt.Scan(&outputMode)
	if err != nil {
		log.Fatalf("Invalid input, expected a number: %v", err)
	}

	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodPublish)
	hasCalEvents := false

	rawText, err := readPdf("Exams.pdf")

	if err != nil {
		log.Fatalf("Failed to read %v", err)
	}
	// Pre-compile regular expressions to dynamically match layout shapes instead of hardcoded index offsets
	dateRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRegex := regexp.MustCompile(`\d{2}:\d{2}`)
	daysOfWeek := map[string]bool{
		"SUNDAY": true, "MONDAY": true, "TUESDAY": true,
		"WEDNESDAY": true, "THURSDAY": true, "FRIDAY": true, "SATURDAY": true,
	}

	// we filtering the raw

	words := strings.Fields(rawText)

	// Loop using an index so we can check neighboring words
	for i := 0; i < len(words); i++ {
		// Check if words[i] matches any course code in out list
		if isTargetCourse(words[i], targetCourses) {
			fmt.Printf("Found target course! %s\n", words[i])

			// Now we scan forward from here to find the end of this exam section
			for j := i + 1; j < len(words); j++ {
				if words[j] == "College" || words[j] == "Online" {

					// Slice out all messy structural text between the code and location marker

					blockWords := words[i+1 : j]
					var titleParts []string
					var examDate, examDay, examTime, amPm string

					// Dynamically classify each string token inside the row segment

					for _, w := range blockWords {
						cleanWord := strings.Trim(w, "| ")
						if cleanWord == "" {
							continue
						}

						// Match explicit date pattern (e.g., 2026-06-15)

						if dateRegex.MatchString(cleanWord) {
							examDate = cleanWord
							continue
						}

						// Match text token against known global calendar days

						if daysOfWeek[strings.ToUpper(cleanWord)] {
							examDay = cleanWord
							continue
						}

						// Isolate AM/PM indicators to reconstruct accurate parse templates

						if strings.ToUpper(cleanWord) == "AM" || strings.ToUpper(cleanWord) == "PM" {
							amPm = cleanWord
							continue
						}
						// Accumulate multiple time stamps safely (handles missing or stray separator dashes)

						if timeRegex.MatchString(cleanWord) {
							if examTime == "" {
								examTime = cleanWord
							} else {
								examTime = examTime + "-" + cleanWord
							}
							continue
						}

						// Filter out PDF structural filler; everything else belongs to the title

						if cleanWord != "At" {
							titleParts = append(titleParts, cleanWord)
						}
					}

					// Explicitly re-attach AM/PM marker to the extracted time segment

					if amPm != "" && examTime != "" {
						examTime = examTime + " " + amPm
					}
					currentExam := exam{
						courseCode:  words[i],
						courseTitle: strings.Join(titleParts, " "),
						examDate:    examDate,
						examDay:     examDay,
						examTime:    examTime,
						examPlace:   words[j],
					}
					// Restore complete location text safely if it was split across fields

					if j > 0 && words[j-1] == "At" {
						currentExam.examPlace = "At " + words[j]
					}

					if outputMode == 2 {

						// Now we gonna create a unique event for each course
						event := cal.AddEvent(fmt.Sprintf("%d-%s@examfinder.local", time.Now().UnixNano(), currentExam.courseCode))
						event.SetSummary(currentExam.courseTitle)
						event.SetDescription("Final Exam")
						event.SetLocation(currentExam.examPlace)
						startTime, endTime, err := parseExamTimes(currentExam.examDate, currentExam.examTime)
						if err != nil {
							log.Fatalf("Error prasing exam time for %s: %v , So we gonna go for the creattion of the event time", currentExam.courseCode, err)
							startTime = time.Now()
							endTime = time.Now().Add(2 * time.Hour)
						}
						event.SetStartAt(startTime)
						event.SetEndAt(endTime)
						hasCalEvents = true
						fmt.Printf("➕ Added %s to calendar stream.\n", currentExam.courseCode)

					} else {

						fmt.Println("\n------")
						fmt.Printf("Code:  %s\n", currentExam.courseCode)
						fmt.Printf("Title: %s\n", currentExam.courseTitle)
						fmt.Printf("Date:  %s (%s)\n", currentExam.examDate, currentExam.examDay)
						fmt.Printf("Time:  %s\n", currentExam.examTime)
						fmt.Printf("Place: %s\n", currentExam.examPlace)
					}

					// Move our outer loop index forward so we don't re-parse inside this block

					i = j
					break
				}
			}
		}
	}
	if outputMode == 2 && hasCalEvents {
		filename := "my_exam_schedule.ics"
		err = os.WriteFile(filename, []byte(cal.Serialize()), 0644)
		if err != nil {
			log.Fatalf("Error saving collective ICS file: %v", err)
		}
		fmt.Printf("\n Success! All exams compiled into a single file: %s\n", filename)
	}

}

func parseExamTimes(dateStr, timeStr string) (time.Time, time.Time, error) {

	// Define a regular expression to look for hour patterns (e.g., 09:00 or 11:00)
	timeRegex := regexp.MustCompile(`\d{2}:\d{2}`)

	// Using -1 extracts all distinct time groupings matching the pattern without layout constraints

	times := timeRegex.FindAllString(timeStr, -1)
	if len(times) == 0 || dateStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date/time inputs (date: '%s', time: '%s')", dateStr, timeStr)
	}

	startHM := times[0] // Always the first time found
	var endHM string

	if len(times) >= 2 {
		endHM = times[1] // The second time found
	} else {
		// Fallback for cases ("• 03:00 PM") where only one time shows up.
		// We'll set the exam to automatically end 2 hours after it starts.
		endHM = startHM
	}

	// Check for AM/PM marker safely
	amPM := "AM"
	if strings.Contains(strings.ToUpper(timeStr), "PM") {
		amPM = "PM"
	}

	startTemplate := fmt.Sprintf("%s %s %s", dateStr, startHM, amPM) // "2026-06-14 09:00 AM"
	endTemplate := fmt.Sprintf("%s %s %s", dateStr, endHM, amPM)     // "2026-06-14 11:00 AM"

	// The New Stencil Layout
	// Since the date format is Year-Month-Day, the template layout must match
	// using Go's magic reference parameters: 2006 (Year), 01 (Month), 02 (Day).
	layout := "2006-01-02 03:04 PM"

	startTime, err := time.ParseInLocation(layout, startTemplate, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	endTime, err := time.ParseInLocation(layout, endTemplate, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if len(times) < 2 { // Safety fallback: If only one time was printed (e.g. "• 03:00 PM"), auto-expire after 2 hours
		endTime = startTime.Add(2 * time.Hour)
	}

	return startTime, endTime, nil

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
