package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/0xb0nzi/htb-cli/config"
	"github.com/0xb0nzi/htb-cli/lib/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type Response struct {
	Message   string `json:"message"`
	ExpiresAt string `json:"expires_at"`
}

// fetchData retrieves a section of profile data from the given API base and
// endpoint, returning the raw value under infoKey. Callers decide how to
// interpret it (a map for the v4 progress endpoints, an array for v5 activity).
func fetchData(baseURL string, itemID int, endpoint string, infoKey string) (interface{}, error) {
	url := fmt.Sprintf("%s%s%d", baseURL, endpoint, itemID)
	config.GlobalConfig.Logger.Debug(fmt.Sprintf("URL: %s", url))

	resp, err := utils.HtbRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return utils.ParseJsonMessage(resp, infoKey), nil
}

// fetchUserProfilePanels gathers the fortress, pro-lab and activity sections
// shown on a user profile, keyed by panel name for the TUI. Failures on an
// individual section are reported but don't abort the others.
func fetchUserProfilePanels(userID int) map[string]map[string]interface{} {
	endpoints := []struct {
		name    string
		baseURL string
		url     string
		infoKey string
	}{
		{"Fortresses", config.BaseHackTheBoxAPIURL, "/user/profile/progress/fortress/", "profile"},
		{"Prolabs", config.BaseHackTheBoxAPIURL, "/user/profile/progress/prolab/", "profile"},
		// Activity was removed from v4 and now lives under v5, returning a
		// flat {"data": [...]} array instead of a nested profile object.
		{"Activity", config.BaseHackTheBoxAPIURLv5, "/user/profile/activity/", "data"},
	}

	dataMaps := make(map[string]map[string]interface{})
	for _, ep := range endpoints {
		raw, err := fetchData(ep.baseURL, userID, ep.url, ep.infoKey)
		if err != nil {
			fmt.Printf("Error fetching data for %s: %v\n", ep.name, err)
			continue
		}

		if ep.name == "Activity" {
			// Wrap the v5 array so the GUI, which reads
			// dataMaps["Activity"]["activity"], finds it where it expects.
			dataMaps[ep.name] = map[string]interface{}{"activity": raw}
			continue
		}

		profile, ok := raw.(map[string]interface{})
		if !ok {
			fmt.Printf("Error fetching data for %s: unexpected response format\n", ep.name)
			continue
		}
		dataMaps[ep.name] = profile
	}
	return dataMaps
}

// fetchAndDisplayInfo fetches and displays information based on the specified parameters.
func fetchAndDisplayInfo(url, header string, params []string, elementType string) error {
	w := utils.SetTabWriterHeader(header)

	// Iteration on all machines / challenges / users argument
	var itemID int
	for _, param := range params {
		if elementType == "Challenge" {
			config.GlobalConfig.Logger.Info("Challenge search...")
			challenges, err := utils.SearchChallengeByName(param)
			if err != nil {
				return err
			}
			config.GlobalConfig.Logger.Debug(fmt.Sprintf("Challenge found: %v", challenges))

			itemID = challenges.ID
		} else {
			itemID, _ = utils.SearchItemIDByName(param, elementType)
		}

		resp, err := utils.HtbRequest(http.MethodGet, fmt.Sprintf("%s%d", url, itemID), nil)
		if err != nil {
			return err
		}

		var infoKey string
		if strings.Contains(url, "machine") {
			infoKey = "info"
		} else if strings.Contains(url, "challenge") {
			infoKey = "challenge"
		} else if strings.Contains(url, "user/profile") {
			infoKey = "profile"
		} else {
			return fmt.Errorf("infoKey not defined")
		}

		config.GlobalConfig.Logger.Debug(fmt.Sprintf("URL: %s", url))
		config.GlobalConfig.Logger.Debug(fmt.Sprintf("InfoKey: %s", infoKey))

		info := utils.ParseJsonMessage(resp, infoKey)
		data := info.(map[string]interface{})

		var bodyData string
		var challengeDescription string
		if elementType == "Machine" {
			status := utils.SetStatus(data)
			retiredStatus := getMachineStatus(data)
			release_key := "release"
			datetime, err := utils.ParseAndFormatDate(data[release_key].(string))
			if err != nil {
				return err
			}
			ip := getIPStatus(data)
			bodyData = fmt.Sprintf("%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", data["name"], data["os"], retiredStatus, data["difficultyText"], data["stars"], ip, status, data["last_reset_time"], datetime)
		} else if elementType == "Challenge" {
			retiredStatus := getMachineStatus(data)
			release_key := "release_date"
			datetime, err := utils.ParseAndFormatDate(data[release_key].(string))
			if err != nil {
				return err
			}
			solved := "No"
			if v, ok := data["authUserSolve"].(bool); ok && v {
				solved = "Yes"
			}
			bodyData = fmt.Sprintf("%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
				data["name"], data["category_name"], data["difficulty"], data["points"], data["solves"], solved, retiredStatus, datetime)

			// Capture the prose synopsis and first blood to print under the table.
			if desc, ok := data["description"].(string); ok {
				challengeDescription = desc
			}
			if blood, ok := data["first_blood_user"].(string); ok && blood != "" {
				challengeDescription += "\nFirst blood: " + blood
			}
		} else if elementType == "Username" {
			// The fortress / pro-lab / activity panels are only rendered for
			// user profiles, so fetch them here rather than for every lookup type.
			utils.DisplayInformationsGUI(data, fetchUserProfilePanels(itemID))
			return nil
		}

		utils.SetTabWriterData(w, bodyData)
		w.Flush()

		if challengeDescription != "" {
			fmt.Printf("\nDescription: %s\n", challengeDescription)
		}
	}
	return nil
}

// coreInfoCmd is the core of the info command; it checks the parameters and displays corresponding information.
func coreInfoCmd(machineName []string, challengeName []string, usernameName []string) error {
	machineHeader := "Name\tOS\tRetired\tDifficulty\tStars\tIP\tStatus\tLast Reset\tRelease"
	challengeHeader := "Name\tCategory\tDifficulty\tPoints\tSolves\tSolved\tRetired\tRelease"
	usernameHeader := "Name\tUser Owns\tSystem Owns\tUser Bloods\tSystem Bloods\tTeam\tUniversity\tRank\tGlobal Rank\tPoints"

	type infoType struct {
		APIURL string
		Header string
		Params []string
		Name   string
	}

	infos := []infoType{
		{config.BaseHackTheBoxAPIURL + "/machine/profile/", machineHeader, machineName, "Machine"},
		{config.BaseHackTheBoxAPIURL + "/challenge/info/", challengeHeader, challengeName, "Challenge"},
		{config.BaseHackTheBoxAPIURL + "/user/profile/basic/", usernameHeader, usernameName, "Username"},
	}

	// No arguments provided
	if len(machineName) == 0 && len(usernameName) == 0 && len(challengeName) == 0 {
		isConfirmed := utils.AskConfirmation("Do you want to check for active machine ?")
		if isConfirmed {
			err := displayActiveMachine(machineHeader)
			if err != nil {
				return err
			}
		}
		// Get current account
		resp, err := utils.HtbRequest(http.MethodGet, config.BaseHackTheBoxAPIURL+"/user/info", nil)
		if err != nil {
			return err
		}
		info := utils.ParseJsonMessage(resp, "info")
		infoMap, _ := info.(map[string]interface{})
		newInfo := infoType{
			APIURL: config.BaseHackTheBoxAPIURL + "/user/profile/basic/",
			Header: "",
			Params: []string{infoMap["name"].(string)},
			Name:   "Username",
		}
		infos = append(infos, newInfo)
	}

	for _, info := range infos {
		if len(info.Params) > 0 {
			if info.Name == "Machine" {
				isConfirmed := utils.AskConfirmation("Do you want to check for active " + strings.ToLower(info.Name) + "?")
				if isConfirmed {
					err := displayActiveMachine(info.Header)
					if err != nil {
						return err
					}
				}
			}
			for _, param := range info.Params {
				err := fetchAndDisplayInfo(info.APIURL, info.Header, []string{param}, info.Name)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// getMachineStatus returns machine status
func getMachineStatus(data map[string]interface{}) string {
	if data["retired"] == false {
		return "No"
	}
	return "Yes"
}

// getIPStatus returns ip status
func getIPStatus(data map[string]interface{}) interface{} {
	if data["ip"] == nil {
		return "Undefined"
	}
	return data["ip"]
}

// displayActiveMachine displays information about the active machine if one is found.
func displayActiveMachine(header string) error {
	machineID, err := utils.GetActiveMachineID()
	if err != nil {
		return err
	}
	if machineID == 0 {
		fmt.Println("No machine is running")
		return nil
	}
	machineType, err := utils.GetMachineType(machineID)
	if err != nil {
		return err
	}
	config.GlobalConfig.Logger.Debug(fmt.Sprintf("Machine Type: %s", machineType))

	expiresTime, err := utils.GetExpiredTime(machineType)
	if err != nil {
		return err
	}
	config.GlobalConfig.Logger.Debug(fmt.Sprintf("Expires Time: %s", expiresTime))

	config.GlobalConfig.Logger.Info("Active machine found !")
	config.GlobalConfig.Logger.Debug(fmt.Sprintf("Machine ID: %d", machineID))

	if expiresTime != "Undefined" {
		err = checkIfExpiringSoon(expiresTime, machineID)
		if err != nil {
			return err
		}
	}

	tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.Debug)
	w := utils.SetTabWriterHeader(header)

	url := fmt.Sprintf("%s/machine/profile/%d", config.BaseHackTheBoxAPIURL, machineID)
	resp, err := utils.HtbRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	info := utils.ParseJsonMessage(resp, "info")
	// info := utils.ParseJsonMessage(resp, "data")

	data := info.(map[string]interface{})
	status := utils.SetStatus(data)
	retiredStatus := getMachineStatus(data)

	datetime, err := utils.ParseAndFormatDate(data["release"].(string))
	if err != nil {
		return err
	}
	config.GlobalConfig.Logger.Debug(fmt.Sprintf("Machine Type: %s", machineType))

	userSubscription, err := utils.GetUserSubscription()
	if err != nil {
		return err
	}
	config.GlobalConfig.Logger.Debug(fmt.Sprintf("User subscription: %s", userSubscription))

	ip := "Undefined"
	_ = ip
	switch {
	case machineType == "release":
		ip, err = utils.GetActiveReleaseArenaMachineIP()
		if err != nil {
			return err
		}
	case userSubscription == "vip+":
		ip, err = utils.GetActiveMachineIP()
		if err != nil {
			return err
		}
	default:
		ip = getIPStatus(data).(string)
	}

	bodyData := fmt.Sprintf("%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
		data["name"], data["os"], retiredStatus,
		data["difficultyText"], data["stars"],
		ip, status, data["last_reset_time"], datetime)

	utils.SetTabWriterData(w, bodyData)
	w.Flush()
	return nil
}

func checkIfExpiringSoon(expiresTime string, machineID int) error {
	layout := "2006-01-02 15:04:05"

	date, err := time.Parse(layout, expiresTime)
	if err != nil {
		return fmt.Errorf("date conversion error: %v", err)
	}

	now := time.Now()
	config.GlobalConfig.Logger.Debug(fmt.Sprintf("Actual date: %v", now))

	timeLeft := date.Sub(now)
	limit := 2 * time.Hour
	if timeLeft > 0 && timeLeft <= limit {
		var remainingTime string
		if date.After(now) {
			duration := date.Sub(now)
			hours := int(duration.Hours())
			minutes := int(duration.Minutes()) % 60
			seconds := int(duration.Seconds()) % 60

			remainingTime = fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)

		}
		// Extend time
		isConfirmed := utils.AskConfirmation(fmt.Sprintf("Would you like to extend the active machine time ? Remaining: %s", remainingTime))
		if isConfirmed {
			jsonData := []byte(fmt.Sprintf(`{"machine_id":%d}`, machineID))
			resp, err := utils.HtbRequest(http.MethodPost, config.BaseHackTheBoxAPIURL+"/vm/extend", jsonData)
			if err != nil {
				return err
			}
			var response Response
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				return fmt.Errorf("error decoding JSON response: %v", err)
			}

			inputLayout := "2006-01-02 15:04:05"

			date, err := time.Parse(inputLayout, response.ExpiresAt)
			if err != nil {
				return fmt.Errorf("error decoding JSON response: %v", err)
			}

			outputLayout := "2006-01-02 - 15h 04m 05s"

			formattedDate := date.Format(outputLayout)

			fmt.Println(response.Message)
			fmt.Printf("Expires Date: %s\n", formattedDate)

		}
	}
	return nil
}

// infoCmd is a Cobra command that serves as an entry point to display detailed information about machines.
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Detailed information on challenges and machines",
	Long:  "Displays detailed information on machines and challenges in a structured table",
	Run: func(cmd *cobra.Command, args []string) {
		machineParam, err := cmd.Flags().GetStringSlice("machine")
		if err != nil {
			fmt.Println(err)
			return
		}

		challengeParam, err := cmd.Flags().GetStringSlice("challenge")
		if err != nil {
			fmt.Println(err)
			return
		}

		usernameParam, err := cmd.Flags().GetStringSlice("username")
		if err != nil {
			fmt.Println(err)
			return
		}
		err = coreInfoCmd(machineParam, challengeParam, usernameParam)
		if err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}
	},
}

// init adds the info command to the root command and sets flags specific to this command.
func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().StringSliceP("machine", "m", []string{}, "Machine name")
	infoCmd.Flags().StringSliceP("challenge", "c", []string{}, "Challenge name")
	infoCmd.Flags().StringSliceP("username", "u", []string{}, "Username")
}
