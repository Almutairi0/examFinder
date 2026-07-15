package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
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
	fileFlag := flag.String("file", "", "path to the exam schedule PDF (skips the interactive file picker)")
	coursesFlag := flag.String("courses", "", "comma-separated course codes, e.g. MATH101,CS211")
	formatFlag := flag.String("format", "text", "output format: text or ics")
	flag.Parse()

	if *fileFlag != "" {
		runNonInteractive(*fileFlag, *coursesFlag, *formatFlag)
		return
	}

	runInteractive()
}

// runNonInteractive drives the flag-based flow:
//
//	examsFinder --file Exams.pdf --courses MATH101,CS211 --format ics
func runNonInteractive(pdfPath, coursesCSV, format string) {
	if coursesCSV == "" {
		log.Fatalf("--courses is required when --file is set (e.g. --courses MATH101,CS211)")
	}

	var targetCourses []string
	for _, c := range strings.Split(coursesCSV, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			targetCourses = append(targetCourses, c)
		}
	}

	rawText, err := readPdf(pdfPath)
	if err != nil {
		log.Fatalf("Failed to read %v: %v", pdfPath, err)
	}

	exams := extractExams(rawText, targetCourses)

	switch strings.ToLower(format) {
	case "ics":
		writeICS(exams, "my_exam_schedule.ics")
	default:
		printExamsToTerminal(exams)
	}
}

// runInteractive keeps the original prompt-driven flow, for anyone who runs
// examsFinder with no flags at all.
func runInteractive() {
	targetFolder := "."
	pdfFiles := find(targetFolder, ".pdf")

	if len(pdfFiles) == 0 {
		log.Fatalf("There is no pdf files to read")
	}

	var selectedPDF string
	scanner := bufio.NewScanner(os.Stdin)

	if len(pdfFiles) > 1 {
		fmt.Println("There is more than one file choose one")
		for i, path := range pdfFiles {
			fmt.Printf("%v %v\n", i+1, path)
		}
		fmt.Println("\nEnter a Number")
		scanner.Scan()
		choiceStr := strings.TrimSpace(scanner.Text())
		choice, err := strconv.Atoi(choiceStr)
		if err != nil || choice < 1 || choice > len(pdfFiles) {
			log.Fatalf("Invalid selection")
		}
		selectedPDF = pdfFiles[choice-1]
	} else {
		selectedPDF = pdfFiles[0]
	}

	fmt.Printf("\nTargeting file: %s\n\n", selectedPDF)

	var targetCourses []string
	fmt.Println("Enter your course codes (press Enter on a blank line to start):")
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			break
		}
		targetCourses = append(targetCourses, input)
		fmt.Printf("Your courses: %v\n", targetCourses)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input standard error: %v", err)
	}

	var outputMode int
	fmt.Println("\nChoose 1 for terminal version and 2 for ics version")
	_, err := fmt.Scan(&outputMode)
	if err != nil {
		log.Fatalf("Invalid input, expected a number: %v", err)
	}

	rawText, err := readPdf(selectedPDF)
	if err != nil {
		log.Fatalf("Failed to read %v: %v", selectedPDF, err)
	}

	exams := extractExams(rawText, targetCourses)

	if outputMode == 2 {
		writeICS(exams, "my_exam_schedule.ics")
	} else {
		printExamsToTerminal(exams)
	}
}

// extractExams scans the raw PDF text word-by-word. For every occurrence of
// one of targetCourses, it pulls the exam block up to the next "College" or
// "Online" location marker and classifies each token inside it (date, day,
// time, AM/PM, or title) - same logic as the original main(), just pulled
// into its own function so both run modes can share it.
func extractExams(rawText string, targetCourses []string) []exam {
	dateRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRegex := regexp.MustCompile(`\d{2}:\d{2}`)
	daysOfWeek := map[string]bool{
		"SUNDAY": true, "MONDAY": true, "TUESDAY": true,
		"WEDNESDAY": true, "THURSDAY": true, "FRIDAY": true, "SATURDAY": true,
	}

	words := strings.Fields(rawText)
	var exams []exam

	for i := 0; i < len(words); i++ {
		if !isTargetCourse(words[i], targetCourses) {
			continue
		}
		fmt.Printf("Found target course! %s\n", words[i])

		for j := i + 1; j < len(words); j++ {
			if words[j] != "College" && words[j] != "Online" {
				continue
			}

			blockWords := words[i+1 : j]
			var titleParts []string
			var examDate, examDay, examTime, amPm string

			for _, w := range blockWords {
				cleanWord := strings.Trim(w, "| ")
				if cleanWord == "" {
					continue
				}
				if dateRegex.MatchString(cleanWord) {
					examDate = cleanWord
					continue
				}
				if daysOfWeek[strings.ToUpper(cleanWord)] {
					examDay = cleanWord
					continue
				}
				if strings.ToUpper(cleanWord) == "AM" || strings.ToUpper(cleanWord) == "PM" {
					amPm = cleanWord
					continue
				}
				if timeRegex.MatchString(cleanWord) {
					if examTime == "" {
						examTime = cleanWord
					} else {
						examTime = examTime + "-" + cleanWord
					}
					continue
				}
				if cleanWord != "At" {
					titleParts = append(titleParts, cleanWord)
				}
			}

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
			if j > 0 && words[j-1] == "At" {
				currentExam.examPlace = "At " + words[j]
			}

			exams = append(exams, currentExam)
			i = j
			break
		}
	}

	return exams
}

func printExamsToTerminal(exams []exam) {
	for _, currentExam := range exams {
		fmt.Println("\n------")
		color.Cyan("Code:   %s\n", currentExam.courseCode)
		color.Blue("Title:  %s\n", currentExam.courseTitle)

		examTimeObj, err := time.Parse("2006-01-02", currentExam.examDate)
		if err != nil {
			color.Green("Date:   %s (%s)\n", currentExam.examDate, currentExam.examDay)
		} else {
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			daysLeft := int(examTimeObj.Sub(today).Hours() / 24)
			color.Green("Date:   %s (%s) [%d days left]\n", currentExam.examDate, currentExam.examDay, daysLeft)
		}

		color.White("Time:   %s\n", currentExam.examTime)
		color.Yellow("Place:  %s\n", currentExam.examPlace)
	}
}

func writeICS(exams []exam, filename string) {
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodPublish)
	hasCalEvents := false

	for _, currentExam := range exams {
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
	}

	if !hasCalEvents {
		fmt.Println("No matching exams found - nothing to write.")
		return
	}

	err := os.WriteFile(filename, []byte(cal.Serialize(ics.WithNewLineWindows)), 0644)
	if err != nil {
		log.Fatalf("Error saving collective ICS file: %v", err)
	}
	fmt.Printf("\n Success! All exams compiled into a single file: %s\n", filename)
}

func parseExamTimes(dateStr, timeStr string) (time.Time, time.Time, error) {
	timeRegex := regexp.MustCompile(`\d{2}:\d{2}`)
	times := timeRegex.FindAllString(timeStr, -1)
	if len(times) == 0 || dateStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date/time inputs (date: '%s', time: '%s')", dateStr, timeStr)
	}

	startHM := times[0]
	var endHM string
	if len(times) >= 2 {
		endHM = times[1]
	} else {
		endHM = startHM
	}

	amPM := "AM"
	if strings.Contains(strings.ToUpper(timeStr), "PM") {
		amPM = "PM"
	}

	startTemplate := fmt.Sprintf("%s %s %s", dateStr, startHM, amPM)
	endTemplate := fmt.Sprintf("%s %s %s", dateStr, endHM, amPM)

	layout := "2006-01-02 03:04 PM"

	startTime, err := time.ParseInLocation(layout, startTemplate, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	endTime, err := time.ParseInLocation(layout, endTemplate, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if len(times) < 2 {
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
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
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
