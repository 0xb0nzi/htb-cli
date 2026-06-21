package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/0xb0nzi/htb-cli/config"
	"github.com/0xb0nzi/htb-cli/lib/output"
	"github.com/0xb0nzi/htb-cli/lib/utils"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	// per_page=100 keeps pagination to a few requests; retired has hundreds of
	// machines, and per_page=20 meant 30+ sequential calls (slow + rate-limit
	// prone). fetchAllPages still walks any remaining pages.
	machineURL = config.BaseHackTheBoxAPIURL + "/machine/paginated/?per_page=100"
	retiredURL = config.BaseHackTheBoxAPIURL + "/machine/list/retired/paginated/?per_page=100&sort_by=release-date"
	// HTB removed the v4 /machine/unreleased route; the unreleased listing now
	// lives under the unified v5 /machines endpoint filtered by state.
	scheduledURL   = config.BaseHackTheBoxAPIURLv5 + "/machines?state=unreleased&per_page=100"
	activeTitle    = "Active"
	retiredTitle   = "Retired"
	scheduledTitle = "Scheduled"
	CheckMark      = "\U00002705"
	CrossMark      = "\U0000274C"
	Penguin        = "\U0001F427"
	Computer       = "\U0001F5A5 "
)

// getColorFromDifficultyText returns the color corresponding to the given difficulty.
func getColorFromDifficultyText(difficultyText string) string {
	switch difficultyText {
	case "Medium":
		return "[orange]"
	case "Easy":
		return "[green]"
	case "Hard":
		return "[red]"
	case "Insane":
		return "[purple]"
	default:
		return "[-]"
	}
}

// getOSEmoji returns an emoji corresponding to the given operating system.
// The comparison is case-insensitive because v4 returns "Windows"/"Linux"
// while the v5 listing returns "windows"/"linux".
func getOSEmoji(os string) string {
	switch strings.ToLower(os) {
	case "linux":
		return Penguin
	case "windows":
		return Computer
	default:
		return ""
	}
}

// createFlex creates and returns a Flex view with machine information
func createFlex(info interface{}, title string, isScheduled bool) (*tview.Flex, error) {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.SetBorder(true).SetTitle(title).SetTitleAlign(tview.AlignLeft)

	for _, value := range info.([]interface{}) {
		data := value.(map[string]interface{})

		// Determining the color according to difficulty

		// Both the v4 active/retired lists and the v5 unreleased list expose the
		// difficulty as "difficultyText".
		key := data["difficultyText"].(string)
		color := getColorFromDifficultyText(key)
		osEmoji := getOSEmoji(data["os"].(string))

		var formatString string

		// Choice of display format depending on the nature of the information
		if isScheduled {
			formatString = fmt.Sprintf("%-10s %s%-10s %s%-10s[-]",
				data["name"], osEmoji, data["os"], color, data["difficultyText"])
		} else {
			// Convert and format date
			parsedDate, err := time.Parse(time.RFC3339Nano, data["release"].(string))
			if err != nil {
				return nil, fmt.Errorf("error parsing date: %v", err)
			}
			formattedDate := parsedDate.Format("02 January 2006")

			userEmoji := CrossMark + "User"
			if value, ok := data["authUserInUserOwns"]; ok && value != nil {
				if value.(bool) {
					userEmoji = CheckMark + "User"
				}
			}

			rootEmoji := CrossMark + "Root"
			if value, ok := data["authUserInRootOwns"]; ok && value != nil {
				if value.(bool) {
					rootEmoji = CheckMark + "Root"
				}
			}

			formatString = fmt.Sprintf("%-15s %s%-10s %s%-10s[-] %-5v %-5v %-7v %-30s",
				data["name"], osEmoji, data["os"], color, data["difficultyText"],
				data["star"], userEmoji, rootEmoji, formattedDate)
		}

		flex.AddItem(tview.NewTextView().SetText(formatString).SetDynamicColors(true), 1, 0, false)
	}

	return flex, nil
}

// outputMachinesJSON fetches the active, retired and scheduled machine lists and
// prints them as a single JSON document. It powers `htb-cli machines --json`,
// the scriptable counterpart of the TUI view.
func outputMachinesJSON() error {
	// fetchAllPages walks every page so the JSON contains the full lists, not
	// just page 1 (mirrors the menu's pagination).
	fetch := func(url string) ([]map[string]interface{}, error) {
		rows, err := fetchAllPages(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get data from %s: %w", url, err)
		}
		return rows, nil
	}

	active, err := fetch(machineURL)
	if err != nil {
		return err
	}
	retired, err := fetch(retiredURL)
	if err != nil {
		return err
	}
	scheduled, err := fetch(scheduledURL)
	if err != nil {
		return err
	}

	return output.PrintJSON(map[string]interface{}{
		"active":    active,
		"retired":   retired,
		"scheduled": scheduled,
	})
}

var machinesCmd = &cobra.Command{
	Use:   "machines",
	Short: "Displays active / retired machines and next machines to be released",
	Run: func(cmd *cobra.Command, args []string) {
		if config.GlobalConfig.OutputJSON {
			if err := outputMachinesJSON(); err != nil {
				config.GlobalConfig.Logger.Error("", zap.Error(err))
				os.Exit(1)
			}
			return
		}

		app := tview.NewApplication()

		getAndDisplayFlex := func(url, title string, isScheduled bool, flex *tview.Flex) error {
			resp, err := utils.HtbRequest(http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("failed to get data from %s: %w", url, err)
			}

			info := utils.ParseJsonMessage(resp, "data")

			machineFlex, err := createFlex(info, title, isScheduled)
			if err != nil {
				return fmt.Errorf("failed to create flex for %s: %w", title, err)
			}

			flex.AddItem(machineFlex, 0, 1, false)
			return nil
		}

		leftFlex := tview.NewFlex().SetDirection(tview.FlexRow)
		rightFlex := tview.NewFlex().SetDirection(tview.FlexRow)

		if err := getAndDisplayFlex(machineURL, activeTitle, false, leftFlex); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		if err := getAndDisplayFlex(retiredURL, retiredTitle, false, leftFlex); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		if err := getAndDisplayFlex(scheduledURL, scheduledTitle, true, rightFlex); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		rightFlex.AddItem(tview.NewTextView().SetText("").SetDynamicColors(true), 0, 0, false)

		mainFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(leftFlex, 0, 3, false).
			AddItem(rightFlex, 0, 1, false)

		if err := app.SetRoot(mainFlex, true).Run(); err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(machinesCmd)
}
