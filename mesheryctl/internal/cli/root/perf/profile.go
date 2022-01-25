package perf

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
	termbox "github.com/nsf/termbox-go"
	log "github.com/sirupsen/logrus"

	"github.com/ghodss/yaml"
	"github.com/layer5io/meshery/mesheryctl/internal/cli/root/config"
	"github.com/layer5io/meshery/mesheryctl/pkg/utils"
	"github.com/layer5io/meshery/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	pageSize          = 25
	viewSingleProfile bool
)

var profileCmd = &cobra.Command{
	Use:   "profile [profile-name]",
	Short: "List performance profiles",
	Long:  `List all the available performance profiles`,
	Args:  cobra.MinimumNArgs(0),
	Example: `
// List performance profiles (maximum 25 profiles)	
mesheryctl perf profile

// List performance profiles with search (maximum 25 profiles)
mesheryctl perf profile test 2 

// View single performance profile with detailed information
mesheryctl perf profile test --view
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// used for searching performance profile
		var searchString string
		// setting up for error formatting
		cmdUsed = "profile"

		mctlCfg, err := config.GetMesheryCtl(viper.GetViper())
		if err != nil {
			return ErrMesheryConfig(err)
		}

		// handles spaces in args if quoted args passed
		for i, arg := range args {
			args[i] = strings.ReplaceAll(arg, " ", "%20")
		}
		// Merge args to get profile-name
		searchString = strings.Join(args, "%20")

		profiles, _, err := fetchPerformanceProfiles(mctlCfg.GetBaseMesheryURL(), searchString, pageSize, pageNumber-1)
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			log.Info("No Performance Profiles to display")
			return nil
		}

		// get profiles as string arrays for printing tabular format profiles
		data := profilesToStringArrays(profiles)

		// print in json/yaml format
		if outputFormatFlag != "" {
			body, _ := json.Marshal(profiles)
			if outputFormatFlag == "yaml" {
				body, _ = yaml.JSONToYAML(body)
			} else if outputFormatFlag != "json" {
				return ErrInvalidOutputChoice()
			}
			log.Info(string(body))
		} else if !viewSingleProfile { // print all profiles
			utils.PrintToTable([]string{"Name", "ID", "RESULTS", "Load-Generator", "Last-Run"}, data)
		} else { // print single profile
			index := 0
			// if profiles more than one profile, ask for profile index
			if len(profiles) > 1 {
				index, err = userPrompt("profile", "Enter index of the profile", data)
				if err != nil {
					return err
				}
			}

			a := profiles[index]

			fmt.Printf("Name: %v\n", a.Name)
			fmt.Printf("ID: %s\n", a.ID.String())
			fmt.Printf("Total Results: %d\n", a.TotalResults)
			fmt.Printf("Endpoint: %v\n", a.Endpoints[0])
			fmt.Printf("Load Generators: %v\n", a.LoadGenerators[0])
			fmt.Printf("Test run duration: %v\n", a.Duration)
			fmt.Printf("QPS: %d\n", a.QPS)
			fmt.Printf("Service Mesh: %v\n", a.ServiceMesh)
			if a.LastRun != nil {
				fmt.Printf("Last Run: %v\n", a.LastRun.Time.Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("Last Run: %v\n", "nil")
			}
		}

		return nil
	},
}

// Fetch performance profiles
func fetchPerformanceProfiles(baseURL, searchString string, pageSize, pageNumber int) ([]models.PerformanceProfile, []byte, error) {
	client := &http.Client{}
	var response *models.PerformanceProfilesAPIResponse

	url := baseURL + "/api/user/performance/profiles"

	// update the url
	url = fmt.Sprintf("%s?page_size=%d&page=%d", url, pageSize, pageNumber)
	if searchString != "" {
		url = url + "&search=" + searchString
	}

	log.Debug(url)

	req, err := utils.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := client.Do(req)

	// failsafe for having an expired token
	if resp.StatusCode == 302 {
		return nil, nil, ErrExpired()
	}

	if err != nil {
		return nil, nil, ErrFailRequest(err)
	}

	// failsafe for not being authenticated
	if utils.ContentTypeIsHTML(resp) {
		return nil, nil, ErrUnauthenticated()
	}

	// failsafe for the case when a valid uuid v4 is not an id of any pattern (bad api call)
	if resp.StatusCode != 200 {
		return nil, nil, ErrFailReqStatus(resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, utils.PerfError("failed to read response body"))
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, nil, ErrFailUnmarshal(err)
	}

	return response.Profiles, body, nil
}

// add profiles as string arrays to print in a tabular format
func profilesToStringArrays(profiles []models.PerformanceProfile) [][]string {
	var data [][]string

	for _, profile := range profiles {
		// adding profile to data for list output
		if profile.LastRun != nil {
			data = append(data, []string{profile.Name, profile.ID.String(), fmt.Sprintf("%d", profile.TotalResults), profile.LoadGenerators[0], profile.LastRun.Time.Format("2006-01-02 15:04:05")})
		} else {
			data = append(data, []string{profile.Name, profile.ID.String(), fmt.Sprintf("%d", profile.TotalResults), profile.LoadGenerators[0], ""})
		}
	}

	return data
}

func userPrompt(key string, label string, data [][]string) (int, error) {
	err := termbox.Init()
	if err != nil {
		return -1, err
	}
	for i, a := range data {
		data[i] = append([]string{strconv.Itoa(i)}, a...)
	}

	if key == "result" {
		utils.PrintToTable([]string{"Index", "Name", "Mesh", "QPS", "Duration", "P50", "P99.9", "Start-Time"}, data)
	} else {
		utils.PrintToTable([]string{"Index", "Name", "ID", "RESULTS", "Load-Generator", "Last-Run"}, data)
	}

	fmt.Printf("\n")
	validate := func(input string) error {
		index, err := strconv.Atoi(input)
		if err != nil {
			return err
		}
		if index < 0 || index >= len(data) {
			return errors.New("Invalid index")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    label,
		Validate: validate,
	}

	result, err := prompt.Run()

	if err != nil {
		termbox.Close()
		return -1, fmt.Errorf("prompt failed %v", err)
	}

	termbox.Close()

	index, err := strconv.Atoi(result)
	if err != nil {
		return -1, err
	}
	return index, nil
}

func init() {
	profileCmd.Flags().BoolVarP(&viewSingleProfile, "view", "", false, "(optional) View single performance profile with more info")
	profileCmd.Flags().IntVarP(&pageNumber, "page", "p", 1, "(optional) List next set of performance results with --page (default = 1)")
}
