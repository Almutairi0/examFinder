package main

import (
	"io/fs"
	"path/filepath"

	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/fatih/color"
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

	targetFolder := "."

	pdfFiles := find(targetFolder, ".pdf")

	if len(pdfFiles) == 0 {
		log.Fatalf("There is no pdf files to read")
	}

	var selectedPDF string

	scanner := bufio.NewScanner(os.Stdin)

	// If multiple PDFs exist, prompt the user to select one via a numbered menu.

	if len(pdfFiles) > 1 {
		fmt.Println("There is more than one file choose one")

		for i, path := range pdfFiles {
			// Print formatted list: [1] file.pdf
			fmt.Printf("%v %v\n", i+1, path)
		}

		fmt.Println("\nEnter a Number")
		scanner.Scan()
		choiceStr := strings.TrimSpace(scanner.Text())
		choice, err := strconv.Atoi(choiceStr) // Convert user input to integer

		if err != nil || choice < 1 || choice > len(pdfFiles) {
			log.Fatalf("Invalid selection")
		}

		// Subtract 1 because slice indices start at 0, but our menu started at 1

		selectedPDF = pdfFiles[choice-1]
	} else {
		// Auto-select if only one PDF is found
		selectedPDF = pdfFiles[0]
	}

	fmt.Printf("\nTargeting file: %s\n\n", selectedPDF)

	var targetCourses []string

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

	// ADD THIS CHECK HERE TO FIX THE WARNING:
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input standard error: %v", err)
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

	rawText, err := readPdf(selectedPDF)
	if err != nil {
		log.Fatalf("Failed to read %v: %v", selectedPDF, err)
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
		// Check if words[i] matches any course code in our list
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

						// Accumulate multiple time stamps safely
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
						// Create a unique event for each course
						event := cal.AddEvent(fmt.Sprintf("%d-%s@examfinder.local", time.Now().UnixNano(), currentExam.courseCode))
						event.SetSummary(currentExam.courseTitle)
						event.SetDescription("Final Exam")
						event.SetLocation(currentExam.examPlace)
						startTime, endTime, err := parseExamTimes(currentExam.examDate, currentExam.examTime)
						if err != nil {
							log.Printf("Error parsing exam time for %s: %v. Using fallback times.", currentExam.courseCode, err)
							startTime = time.Now()
							endTime = time.Now().Add(2 * time.Hour)
						}
						event.SetStartAt(startTime)
						event.SetEndAt(endTime)
						reminder := ics.NewAlarm("")
						reminder.SetSummary(fmt.Sprintf("%v coming in 2 hours", currentExam.courseTitle))
						reminder.SetDescription(fmt.Sprintf("%v coming in 2 hours", currentExam.courseTitle))
						reminder.SetAction(ics.ActionDisplay)
						reminder.SetTrigger("-PT2H")
						event.AddVAlarm(reminder)
						hasCalEvents = true
						fmt.Printf("Added %s to calendar stream.\n", currentExam.courseCode)

					} else {
						fmt.Println("\n------")
						color.Cyan("Code:   %s\n", currentExam.courseCode)
						color.Blue("Title:  %s\n", currentExam.courseTitle)

						// 1. Convert the exam date string to a time object
						examTimeObj, err := time.Parse("2006-01-02", currentExam.examDate)

						if err != nil {

							// FALLBACK: If the date is unreadable, just print it normally without crashing
							color.Green("Date:   %s (%s)\n", currentExam.examDate, currentExam.examDay)
						} else {

							// 2. Get today's date at midnight for an accurate comparison
							now := time.Now()
							today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

							// 3. Calculate the number of days left
							daysLeft := int(examTimeObj.Sub(today).Hours() / 24)

							// 4. Print the date with the countdown
							color.Green("Date:   %s (%s) [%d days left]\n", currentExam.examDate, currentExam.examDay, daysLeft)
						}

						color.White("Time:   %s\n", currentExam.examTime)
						color.Yellow("Place:  %s\n", currentExam.examPlace)
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

	startHM := times[0]
	var endHM string

	if len(times) >= 2 {
		endHM = times[1]
	} else { // Fallback for cases ("• 03:00 PM") where only one time shows up.
		endHM = startHM
	}

	amPM := "AM"
	if strings.Contains(strings.ToUpper(timeStr), "PM") {
		amPM = "PM"
	}

	startTemplate := fmt.Sprintf("%s %s %s", dateStr, startHM, amPM)
	endTemplate := fmt.Sprintf("%s %s %s", dateStr, endHM, amPM)

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

	if len(times) < 2 { //We'll set the exam to automatically end 2 hours after it starts in case of only one time shows up.
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
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

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

// Source - https://stackoverflow.com/a/67629473
// Posted by Zombo
// Retrieved 2026-07-09, License - CC BY-SA 4.0

func find(root, ext string) []string {
	var matches []string

	// WalkDir traverses the file tree starting at the root
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// If the file extension matches our target (e.g., ".pdf"), save its path
		if filepath.Ext(entry.Name()) == ext {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		log.Printf("warning: error walking %q: %v", root, err)
	}
	return matches
}
