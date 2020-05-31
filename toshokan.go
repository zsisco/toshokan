package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
//	"path/filepath"
//	"strconv"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

type Entry struct {
	Filename string
	Title string
	Authors string
	Year string
	Tags []string
	Read bool
	BibText string
	Notes string
}

const (
	READ = iota
	TITLE
	AUTHORS
	YEAR
)

const LIBRARY = "./library/"
const PDF_VIEWER = "mupdf"
const TOSHOKAN = "./toshokan.json"

// Show a navigable tree view of the current directory.
func main() {
	BoolToReadFlag := func(b bool) string {
		if b {
			return "o"
		} else {
			return "-"
		}
	}

	ReadFlagToBool := func(f string) bool {
		if f == "o" {
			return true
		} else {
			return false
		}
	}

	SwapReadFlag := func(f string) string {
		if f == "o" {
			return "-"
		} else {
			return "o"
		}
	}

	CreateCell := func(content string, align int, selectable bool) *tview.TableCell {
		return tview.NewTableCell(content).
			SetAlign(align).
			SetSelectable(selectable)
	}

	WriteToJson := func(toshokan *[]Entry) {
		bytes, merr := json.Marshal(toshokan)
		Check("json.Marshal error", merr)
		werr := ioutil.WriteFile(TOSHOKAN, bytes, 0644)
		Check("WriteFile error", werr)
	}

	// TODO:
	// this should also refresh the library directory 
	// and adding any entries
	ReadFromJson := func(toshokan *[]Entry) {
		toshokanFile, readerr := ioutil.ReadFile(TOSHOKAN)
		Check("ReadFile error", readerr)
		uerr := json.Unmarshal(toshokanFile, &toshokan)
		Check("json.Unmarshal error", uerr)
	}

	RedrawTable := func(table *tview.Table, toshokan []Entry) {
		for row, entry := range toshokan {
			// TODO: check tags with current tag selection (unless "All")
			table.SetCell(row, READ, CreateCell(BoolToReadFlag(entry.Read), tview.AlignLeft, true))
			table.SetCell(row, TITLE, CreateCell(entry.Title, tview.AlignLeft, true))
			table.SetCell(row, AUTHORS, CreateCell(entry.Authors, tview.AlignLeft, true))
			table.SetCell(row, YEAR, CreateCell(entry.Year, tview.AlignLeft, true))
		}
		table.SetBorder(true)
		table.SetTitle("All") // TODO: update with selected tags
		table.SetSelectable(true, false)
	}

	Refresh := func(table *tview.Table, toshokan *[]Entry) {
		ReadFromJson(toshokan)
		RedrawTable(table, *toshokan)
	}

	// Initialze!
	var toshokan []Entry
	
	app := tview.NewApplication()

	table := tview.NewTable().SetFixed(1, 1)

	Refresh(table, &toshokan)

	app.SetFocus(table)

	/* XXX:	This turned into a pretty righteous cluster fuck.
	   [ ] Move table shortcuts to table.SetDoneFunc() like escape and enter and
	       just use control-key shortcuts to minimize interference.
	   [ ] Make the app root a Pages object. Then add the table and form as Pages.
	       This should be easier to switch focus. Check the Pages demo.
	 */
	freeInput := false
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
	/* TODO: 
	   [X] enter: open in pdf viewer
	   [X] r: refresh table view (reload json file) (also tags frame)
	   [ ] e: edit meta data (title, authors, year, tags, filename (readonly))
	   [X] q: toggle read flag
	   [ ] w: open notes file in text editor (create file in not exists); how to open vim inside the program like aerc?
	   [ ] t: open bib file in text editor (create file in not exists)
	   [ ] /: search meta data in current view (moves cursor with n/N search results) [ADVANCED]
	   [ ] J: tag up
	   [ ] K: tag down
	 */

		switch event.Key() {
		case tcell.KeyEscape:
			if freeInput { return event }
			app.Stop()
			return nil
		case tcell.KeyEnter:
			if freeInput { return event }
			row, _ := table.GetSelection()
			selectedFile := toshokan[row].Filename
			cmd := exec.Command(PDF_VIEWER, LIBRARY + selectedFile)
			err := cmd.Start()
			if err != nil {
				panic(err.Error())
				return nil
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'r':
				if freeInput { return event }
				Refresh(table, &toshokan)
				return nil
			case 'e':
				// edit meta data
				// tview.Form with input fields and Save/Cancel
				if freeInput { return event }
				freeInput = true
				row, _ := table.GetSelection()
				newTitle := ""
				newAuthors := ""
				newYear := ""
				metadataForm := tview.NewForm().
					AddInputField("Title", toshokan[row].Title, 0, nil, func(changed string) {
						newTitle = changed
					}).
					AddInputField("Authors", toshokan[row].Authors, 0, nil, func(changed string) {
						newAuthors = changed
					}).
					AddInputField("Year", toshokan[row].Year, 4, nil, func(changed string) {
						newYear = changed
					}).
					AddButton("Save", func() {
						if newTitle != "" {
							toshokan[row].Title = newTitle
						}
						if newAuthors != "" {
							toshokan[row].Authors = newAuthors
						}
						if newYear != "" {
							toshokan[row].Year = newYear
						}
						freeInput = false
						if err := app.SetRoot(table, true).SetFocus(table).Run(); err != nil {
							panic(err)
						}
						Refresh(table, &toshokan)
					}).
					AddButton("Cancel", func() {
						freeInput = false
						if err := app.SetRoot(table, true).SetFocus(table).Run(); err != nil {
							panic(err)
						}
					})
				metadataForm.SetBorder(true).
					SetTitle("File: " + toshokan[row].Filename).
					SetTitleAlign(tview.AlignLeft)
				if err := app.SetRoot(metadataForm, true).SetFocus(metadataForm).Run(); err != nil {
					panic(err)
				}
				freeInput = false
				return nil
			case 'q':
				if freeInput { return event }
				row, _ := table.GetSelection()
				newReadFlag := SwapReadFlag(table.GetCell(row, READ).Text)
				table.SetCell(row, READ, CreateCell(newReadFlag, tview.AlignLeft, true))
				toshokan[row].Read = ReadFlagToBool(newReadFlag)
				WriteToJson(&toshokan)
				return nil
			}
			case 'w':
				if freeInput { return event }
				// open notes in vim
				return nil
			case 't':
				if freeInput { return event }
				// open bibtex in vim
				return nil
			case '/':
				if freeInput { return event }
				// search
				return nil
			case 'J':
				if freeInput { return event }
				return nil
			case 'K':
				if freeInput { return event }
				return nil
			// Fall through to capture table-level input like j,k,h,l,ESC,...
			return event
		}
		return event
	})
	table.SetDoneFunc(func(key tcell.Key) {
		/*if key == tcell.KeyEscape {
			app.Stop()
		}
		if key == tcell.KeyEnter {
			row, _ := table.GetSelection()
			selectedFile := toshokan[row].Filename
			cmd := exec.Command(PDF_VIEWER, LIBRARY + selectedFile)
			err := cmd.Start()
			if err != nil {
				panic(err.Error())
			}
		}*/
	}).SetSelectedFunc(func(row int, column int) {
		/*if column == READ {
			newReadFlag := SwapReadFlag(table.GetCell(row, column).Text)
			table.SetCell(row, column, CreateCell(newReadFlag, tview.AlignLeft, true))
			// TODO: write to JSON (split into generic function)
			// basically update toshokan, call Marshal, check for errors, write []byte to toshokanFile
			toshokan[row].Read = ReadFlagToBool(newReadFlag)
			bytes, merr := json.Marshal(toshokan)
			Check("json.Marshal error", merr)
			werr := ioutil.WriteFile(TOSHOKAN, bytes, 0644)
			Check("WriteFile error", werr)
		} else if column == TITLE {
			table.GetCell(row, column).SetTextColor(tcell.ColorRed)
			table.SetSelectable(false, false)
		}*/
	})

	// TODO: create left frame for tags
	// navigate by J/K
	// selection updates the table title, refreshes the table view with
	// only entries that match the tag

	rooterr := app.SetRoot(table, true).SetFocus(table).Run()
	Check("SetRoot error", rooterr)
}

func Check(msg string, err error) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}
