package action

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/ovh/cds/engine/worker/pkg/workerruntime"
	"github.com/ovh/cds/sdk"
	"github.com/ovh/cds/sdk/cdsclient"
	"github.com/ovh/venom"
)

func RunParseJunitTestResultAction(ctx context.Context, wk workerruntime.Runtime, a *sdk.Action, params []sdk.Parameter, secrets []sdk.Variable) (sdk.Result, error) {
	var res sdk.Result
	res.Status = sdk.StatusFail

	jobID, err := workerruntime.JobID(ctx)
	if err != nil {
		return res, err
	}

	p := sdk.ParameterValue(a.Parameters, "path")
	if p == "" {
		return res, errors.New("UnitTest parser: path not provided")
	}

	files, errg := filepath.Glob(p)
	if errg != nil {
		return res, errors.New("UnitTest parser: Cannot find requested files, invalid pattern")
	}

	var tests venom.Tests
	wk.SendLog(workerruntime.LevelInfo, fmt.Sprintf("%d", len(files))+" file(s) to analyze")

	for _, f := range files {
		var ftests venom.Tests

		data, errRead := ioutil.ReadFile(f)
		if errRead != nil {
			return res, fmt.Errorf("UnitTest parser: cannot read file %s (%s)", f, errRead)
		}

		var vf venom.Tests
		if err := xml.Unmarshal(data, &vf); err != nil {
			// Check if file contains testsuite only (and no testsuites)
			if s, ok := parseTestsuiteAlone(data); ok {
				ftests.TestSuites = append(ftests.TestSuites, s)
			}
			tests.TestSuites = append(tests.TestSuites, ftests.TestSuites...)
		} else {
			tests.TestSuites = append(tests.TestSuites, vf.TestSuites...)
		}
	}

	wk.SendLog(workerruntime.LevelInfo, fmt.Sprintf("%d", len(tests.TestSuites))+" Total Testsuite(s)")
	reasons := computeStats(&res, &tests)
	for _, r := range reasons {
		wk.SendLog(workerruntime.LevelInfo, r)
	}

	if err := wk.Blur(tests); err != nil {
		return res, err
	}

	uri := fmt.Sprintf("/queue/workflows/%d/test", jobID)
	statusCode, errPost := wk.Client().(cdsclient.Raw).PostJSON(ctx, uri, tests, nil)
	if errPost == nil && statusCode > 300 {
		errPost = fmt.Errorf("HTTP %d", statusCode)
	}

	if errPost != nil {
		return res, fmt.Errorf("JUnit parse: failed to send tests details: %s", errPost)
	}

	return res, nil
}

// computeStats computes failures / errors on testSuites,
// set result.Status and return a list of log to send to API
func computeStats(res *sdk.Result, v *venom.Tests) []string {
	// update global stats
	for _, ts := range v.TestSuites {
		nSkipped := 0
		for _, tc := range ts.TestCases {
			nSkipped += len(tc.Skipped)
		}
		if ts.Skipped < nSkipped {
			ts.Skipped = nSkipped
		}
		if ts.Total < len(ts.TestCases)-nSkipped {
			ts.Total = len(ts.TestCases) - nSkipped
		}
		v.Total += ts.Total
		v.TotalOK += ts.Total - ts.Failures - ts.Errors
		v.TotalKO += ts.Failures + ts.Errors
		v.TotalSkipped += ts.Skipped
	}

	var nbOK, nbKO, nbSkipped int

	reasons := []string{}
	reasons = append(reasons, fmt.Sprintf("JUnit parser: %d testsuite(s)", len(v.TestSuites)))

	for i, ts := range v.TestSuites {
		var nbKOTC, nbFailures, nbErrors, nbSkippedTC int
		if ts.Name == "" {
			ts.Name = fmt.Sprintf("TestSuite.%d", i)
		}
		reasons = append(reasons, fmt.Sprintf("JUnit parser: testsuite %s has %d testcase(s)", ts.Name, len(ts.TestCases)))
		for k, tc := range ts.TestCases {
			if tc.Name == "" {
				tc.Name = fmt.Sprintf("TestCase.%d", k)
			}
			if len(tc.Failures) > 0 {
				reasons = append(reasons, fmt.Sprintf("JUnit parser: testcase %s has %d failure(s)", tc.Name, len(tc.Failures)))
				nbFailures += len(tc.Failures)
			}
			if len(tc.Errors) > 0 {
				reasons = append(reasons, fmt.Sprintf("JUnit parser: testcase %s has %d error(s)", tc.Name, len(tc.Errors)))
				nbErrors += len(tc.Errors)
			}
			if len(tc.Failures) > 0 || len(tc.Errors) > 0 {
				nbKOTC++
			} else if len(tc.Skipped) > 0 {
				nbSkippedTC += len(tc.Skipped)
			}
			v.TestSuites[i].TestCases[k] = tc
		}
		nbOK += len(ts.TestCases) - nbKOTC
		nbKO += nbKOTC
		nbSkipped += nbSkippedTC
		if ts.Failures > nbFailures {
			nbFailures = ts.Failures
		}
		if ts.Errors > nbErrors {
			nbErrors = ts.Errors
		}

		if nbFailures > 0 {
			reasons = append(reasons, fmt.Sprintf("JUnit parser: testsuite %s has %d failure(s)", ts.Name, nbFailures))
		}
		if nbErrors > 0 {
			reasons = append(reasons, fmt.Sprintf("JUnit parser: testsuite %s has %d error(s)", ts.Name, nbErrors))
		}
		if nbKOTC > 0 {
			reasons = append(reasons, fmt.Sprintf("JUnit parser: testsuite %s has %d test(s) failed", ts.Name, nbKOTC))
		}
		if nbSkippedTC > 0 {
			reasons = append(reasons, fmt.Sprintf("JUnit parser: testsuite %s has %d test(s) skipped", ts.Name, nbSkippedTC))
		}
		v.TestSuites[i] = ts
	}

	if nbKO > v.TotalKO {
		v.TotalKO = nbKO
	}

	if nbOK != v.TotalOK {
		v.TotalOK = nbOK
	}

	if nbSkipped != v.TotalSkipped {
		v.TotalSkipped = nbSkipped
	}

	if v.TotalKO+v.TotalOK != v.Total {
		v.Total = v.TotalKO + v.TotalOK + v.TotalSkipped
	}

	res.Status = sdk.StatusFail
	if v.TotalKO == 0 {
		res.Status = sdk.StatusSuccess
	}
	return reasons
}

func parseTestsuiteAlone(data []byte) (venom.TestSuite, bool) {
	var s venom.TestSuite
	err := xml.Unmarshal([]byte(data), &s)
	if err != nil {
		return s, false
	}

	if s.Name == "" {
		return s, false
	}

	return s, true
}