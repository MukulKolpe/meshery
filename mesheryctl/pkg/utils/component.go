package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/csv"
	"github.com/layer5io/meshkit/utils/manifests"
)

type ComponentCSV struct {
	Registrant         string `json:"registrant"`
	Model              string `json:"model"`
	Component          string `json:"component"`
	Shape              string `json:"shape"`
	PrimaryColor       string `json:"primaryColor"`
	SecondaryColor     string `json:"secondaryColor"`
	SVGColor           string `json:"svgColor"`
	SVGWhite           string `json:"svgWhite"`
	SVGComplete        string `json:"svgComplete"`
	HasSchema          string `json:"hasSchema"`
	Description        string `json:"description"`
	Docs               string `json:"docs"`
	StyleOverrides     string `json:"styleOverrides"`
	Styles             string `json:"styles"`
	ShapePolygonPoints string `json:"shapePolygonPoints"`
	DefaultData        string `json:"defaultData"`
	Capabilities       string `json:"capabilities"`
	LogoURL            string `json:"logoURL"`
	Genealogy          string `json:"genealogy"`
	IsAnnotation       string `json:"isAnnotation"`
}
type ComponentCSVHelper struct {
	SpreadsheetID  int64
	SpreadsheetURL string
	Title          string
	CSVPath        string
	Components     map[string]map[string][]ComponentCSV
}

func NewComponentCSVHelper(sheetURL, spreadsheetName string, spreadsheetID int64) (*ComponentCSVHelper, error) {
	sheetURL = sheetURL + "/pub?output=csv" + "&gid=" + strconv.FormatInt(spreadsheetID, 10)
	fmt.Println("Downloading CSV from:", sheetURL)
	csvPath := filepath.Join(utils.GetHome(), ".meshery", "content", "components.csv")
	err := utils.DownloadFile(csvPath, sheetURL)
	if err != nil {
		return nil, err
	}

	return &ComponentCSVHelper{
		SpreadsheetID:  spreadsheetID,
		SpreadsheetURL: sheetURL,
		Title:          spreadsheetName,
		CSVPath:        csvPath,
		Components:     make(map[string]map[string][]ComponentCSV),
	}, nil
}

func (mch *ComponentCSVHelper) ParseComponentsSheet() error {
	ch := make(chan ComponentCSV, 1)
	errorChan := make(chan error, 1)
	csvReader, err := csv.NewCSVParser[ComponentCSV](mch.CSVPath, rowIndex, nil, func(_ []string, _ []string) bool {
		return true
	})

	if err != nil {
		return ErrFileRead(err)
	}

	go func(){
		err := csvReader.Parse(ch, errorChan)
		if err != nil {
			errorChan <- err
		}
	}()

	for {
		select {

		case data := <-ch:
			if mch.Components[data.Registrant] == nil {
				mch.Components[data.Registrant] = make(map[string][]ComponentCSV, 0)
			}
			if mch.Components[data.Registrant][data.Model] == nil {
				mch.Components[data.Registrant][data.Model] = make([]ComponentCSV, 0)
			}
			mch.Components[data.Registrant][data.Model] = append(mch.Components[data.Registrant][data.Model], data)
			fmt.Printf("Reading Component: %s for Modal: %s from Registrant: %s\n", data.Component, data.Model, data.Registrant)
		case err := <-errorChan:
			fmt.Println(err)

		case <-csvReader.Context.Done():
			return nil
		}
	}
}

func CreateComponentsMetadataAndCreateSVGsForMDXStyle(components []ComponentCSV, path, svgDir string) (string, error) {
	err := os.MkdirAll(filepath.Join(path, svgDir), 0777)
	if err != nil {
		return "", err
	}
	componentMetadata := `[`
	for idx, comp := range components {
		componentTemplate := `
{
"name": "%s",
"colorIcon": "%s",
"whiteIcon": "%s",
"description": "%s",
}`

		// add comma if not last component
		if idx != len(components)-1 {
			componentTemplate += ","
		}

		compName := utils.FormatName(manifests.FormatToReadableString(comp.Component))
		colorIconDir := filepath.Join(svgDir, compName, "icons", "color")
		whiteIconDir := filepath.Join(svgDir, compName, "icons", "white")

		componentMetadata += fmt.Sprintf(componentTemplate, compName, fmt.Sprintf("%s/%s-color.svg", colorIconDir, compName), fmt.Sprintf("%s/%s-white.svg", whiteIconDir, compName), comp.Description)

		// create color svg dir
		err = os.MkdirAll(filepath.Join(path, colorIconDir), 0777)
		if err != nil {
			return "", err
		}

		// create white svg dir
		err = os.MkdirAll(filepath.Join(path, whiteIconDir), 0777)
		if err != nil {
			return "", err
		}

		err = utils.WriteToFile(filepath.Join(path, colorIconDir, compName+"-color.svg"), comp.SVGColor)
		if err != nil {
			return "", err
		}
		err = utils.WriteToFile(filepath.Join(path, whiteIconDir, compName+"-white.svg"), comp.SVGWhite)
		if err != nil {
			return "", err
		}
	}

	componentMetadata += `]`

	return componentMetadata, nil
}

func CreateComponentsMetadataAndCreateSVGsForMDStyle(components []ComponentCSV, path, svgDir string) (string, error) {
	err := os.MkdirAll(filepath.Join(path), 0777)
	if err != nil {
		return "", err
	}
	componentMetadata := ""
	for _, comp := range components {
		componentTemplate := `
- name: %s
  colorIcon: %s
  whiteIcon: %s
  description: %s`

		compName := utils.FormatName(manifests.FormatToReadableString(comp.Component))
		colorIconDir := filepath.Join(svgDir, compName, "icons", "color")
		whiteIconDir := filepath.Join(svgDir, compName, "icons", "white")

		componentMetadata += fmt.Sprintf(componentTemplate, compName, fmt.Sprintf("%s/%s-color.svg", colorIconDir, compName), fmt.Sprintf("%s/%s-white.svg", whiteIconDir, compName), comp.Description)

		// create color svg dir
		err = os.MkdirAll(filepath.Join(path, compName, "icons", "color"), 0777)
		if err != nil {
			return "", err
		}

		// create white svg dir
		err = os.MkdirAll(filepath.Join(path, compName, "icons", "white"), 0777)
		if err != nil {
			return "", err
		}

		err = utils.WriteToFile(filepath.Join(path, compName, "icons", "color", compName+"-color.svg"), comp.SVGColor)
		if err != nil {
			return "", err
		}
		err = utils.WriteToFile(filepath.Join(path, compName, "icons", "white", compName+"-white.svg"), comp.SVGWhite)
		if err != nil {
			return "", err
		}
	}

	return componentMetadata, nil
}

func (m ComponentCSVHelper) Cleanup() error {
	// remove csv file
	fmt.Println("Removing CSV file:", m.CSVPath)
	err := os.Remove(m.CSVPath)
	if err != nil {
		return err
	}
	return nil
}
