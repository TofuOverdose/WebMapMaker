package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TofuOverdose/WebMapMaker/internal/linkcrawler"
	"github.com/TofuOverdose/WebMapMaker/internal/sitemap"
	"github.com/TofuOverdose/WebMapMaker/internal/utils/gost"
)

type InputData struct {
	TargetURL  string
	OutputPath string
	OutputType string
	Options    []linkcrawler.Option
	LogWriter  io.WriteCloser
}

func main() {
	inputData, err := getInputData()
	if err != nil {
		log.Fatal(err)
	}

	defer inputData.LogWriter.Close()

	results := make([]linkcrawler.SearchResult, 0)

	// Configuring CLI
	type linksDisplayStats struct {
		TotalFoundCount int
		AcceptedCount   int
		FailedCount     int
	}

	linkStats := linksDisplayStats{
		TotalFoundCount: 0,
		AcceptedCount:   0,
		FailedCount:     0,
	}
	sdt := "\t[ {{.AcceptedCount}} accepted | {{.FailedCount}} errors | {{.TotalFoundCount}} total links found ]"

	statsDisplay, err := gost.NewDisplay(sdt, linkStats)
	if err != nil {
		panic(err)
	}

	pb := gost.NewBouncer(10, gost.BouncerCharSet{
		Inactive:    '░',
		Active:      '█',
		BorderLeft:  "|",
		BorderRight: "|",
		Separator:   "|",
	})

	tr := time.Millisecond * 50

	timer := gost.NewTimer(
		gost.TimerOptionShowUnit(true),
		gost.TimerOptionTimeFormatter(gost.TimeFormatterAdaptive),
		gost.TimerOptionSetDecoration(" (time elapsed: ", ") "),
	)

	statusBar := gost.NewStatusBar(tr, pb, statsDisplay, timer)

	jobCtx, jobCancel := context.WithCancel(context.Background())
	stopSigs := make(chan os.Signal)
	signal.Notify(stopSigs, syscall.SIGINT, syscall.SIGTERM)

	resChan, err := linkcrawler.Crawl(jobCtx, inputData.TargetURL, inputData.Options...)
	if err != nil {
		log.Fatal(err)
	}

	statusBar.Run()

	statusBar.Print("Started crawling the website")

	for {
		select {
		case <-stopSigs:
			jobCancel()
			statusBar.Close()
			statusBar.Print("Aborted")
			return
		case res, ok := <-resChan:
			if ok {
				linkStats.TotalFoundCount++
				if res.Error != nil {
					linkStats.FailedCount++
					statusBar.Printf("%s: %s", res.Addr, res.Error.Error())
				} else {
					linkStats.AcceptedCount++
					results = append(results, res)
				}

				// Update display data
				statsDisplay.SetData(linkStats)
			} else {
				//statusBar.Close()
				statusBar.Print("Finished crawling. Building sitemap...")
				us := sitemap.NewUrlSet()

				for _, res := range results {
					us.AddUrl(*sitemap.NewUrl(res.Addr, "", "", 0.0))
				}
				// Open output file
				f, err := os.Create(inputData.OutputPath)
				if err != nil {
					log.Fatal(err)
					return
				}
				defer f.Close()

				switch inputData.OutputType {
				case "XML":
					err = us.WriteXml(f)
				case "TXT":
					err = us.WritePlain(f)
				}
				if err != nil {
					msg := fmt.Sprintf("FATAL: %s\n", err.Error())
					inputData.LogWriter.Write([]byte(msg))
					return
				}
				statusBar.Printf("Sitemap saved to %s", inputData.OutputPath)
				return
			}
		}
	}
}

func getInputData() (*InputData, error) {
	inputData := InputData{}

	// First define the flags
	pTargetURL := flag.String("t", "", "Target URL to start crawling from")
	pOutputPath := flag.String("o", "", "Output file (either TXT or XML)")
	pLogFile := flag.String("log", "", "Path to log file")
	pMaxRoutines := flag.Int("mr", 0, "Set positive number to limit the number of spawned goroutines")
	pSearchOpts := flag.String("sp", "", "Search rules for crawler separated by commas. Available options: ignoreTopLevelDomain, includeWithQuery, includeSubdomains")
	// Then run the parser
	flag.Parse()
	// Validation for the received flags
	if err := validateURL(*pTargetURL); err != nil {
		return nil, err
	}
	inputData.TargetURL = *pTargetURL

	if ot, err := checkOutputFile(*pOutputPath, []string{"XML", "TXT"}); err != nil {
		return nil, err
	} else {
		inputData.OutputPath = *pOutputPath
		inputData.OutputType = ot
	}

	if wc, err := getWriteCloser(*pLogFile); err != nil {
		return nil, err
	} else {
		inputData.LogWriter = wc
	}

	options := make([]linkcrawler.Option, 0)
	if *pMaxRoutines > 0 {
		options = append(options, linkcrawler.OptionMaxRoutines(uint(*pMaxRoutines)))
	}
	searchOptions, err := parseSearchOptions(*pSearchOpts)
	if err != nil {
		return nil, err
	}
	options = append(options, searchOptions...)
	inputData.Options = options

	return &inputData, nil
}

func getWriteCloser(path string) (io.WriteCloser, error) {
	if path == "" {
		return os.Stdout, nil
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func checkOutputFile(path string, allowedTypes []string) (string, error) {
	// Uppercase allowedTypes for convenience
	types := make([]string, len(allowedTypes))
	for i, t := range allowedTypes {
		types[i] = strings.ToUpper(t)
	}

	errTypes := fmt.Errorf("Output file type must be one of these: %s", strings.Join(types, ", "))

	fExt := strings.ToUpper(getExtension(path))
	if fExt == "" {
		return "", errTypes
	}
	// Check if the file extension is among allowed
	found := false
	for _, t := range types {
		if fExt == t {
			found = true
			break
		}
	}
	if !found {
		return "", errTypes
	}

	return fExt, nil
}

func getExtension(path string) string {
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}

func validateURL(urlString string) error {
	errs := make([]string, 0)

	u, err := url.Parse(urlString)
	if err != nil {
		return fmt.Errorf("Invalid target URL: %s", err.Error())
	}

	if u.Scheme == "" {
		errs = append(errs, "scheme (http/https) is required")
	}

	if u.Host == "" {
		errs = append(errs, "hostname is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("Invalid target URL: %s", strings.Join(errs, ", "))
	}

	return nil
}

func parseSearchOptions(input string) ([]linkcrawler.Option, error) {
	if input == "" {
		return nil, nil
	}
	options := make([]linkcrawler.Option, 0)
	input = strings.ReplaceAll(input, " ", "")
	for _, opt := range strings.Split(input, ",") {
		switch opt {
		case "ignoreTopLevelDomain":
			options = append(options, linkcrawler.OptionSearchIgnoreTopLevelDomain())
		case "includeWithQuery":
			options = append(options, linkcrawler.OptionSearchAllowQuery())
		case "includeSubdomains":
			options = append(options, linkcrawler.OptionSearchIncludeSubdomains())
		default:
			return nil, fmt.Errorf("Unsupported search option: %s", opt)
		}
	}
	return options, nil
}
