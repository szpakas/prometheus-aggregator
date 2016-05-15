package main

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type sampleParserState int

const (
	sampleParserStateSearching sampleParserState = iota + 1
	sampleParserStateSample

	sampleParserLabelsSeparator         = ";"
	sampleParserHistogramDefSeparator   = ";"
	sampleParserLabelFromValueSeparator = "="
	sampleParserSamplePartsSeparator    = "|"
)

var (
	labelNameREPart          = `[a-zA-Z0-9]+`
	labelValueREPart         = `[a-zA-Z0-9.]+`
	labelWithValueREPart     = labelNameREPart + sampleParserLabelFromValueSeparator + labelValueREPart
	sampleParserLabelsREPart = `(` + labelWithValueREPart +
		`|` + labelWithValueREPart + `(` + sampleParserLabelsSeparator + labelWithValueREPart + `)+)`

	sampleParserSharedLabelsLineRE = regexp.MustCompile(`^` + sampleParserLabelsREPart + `$`)

	metricNameREPart         = `[a-zA-Z0-9_]+`
	sampleKindREPart         = `[a-z]{1,2}`
	sampleHistogramDefREPart = `[0-9.]+;[0-9.]+;[0-9.]+`
	// TODO(szpakas): tighter regexp with only one decimal separator
	sampleValueREPart            = `[0-9.]+`
	sampleParserSampleLineREPart = `^` +
		metricNameREPart + `\|` +
		sampleKindREPart + `\|` +
		`(` + sampleHistogramDefREPart + `\|)?` + // optional
		`(` + sampleParserLabelsREPart + `\|)?` + // optional
		sampleValueREPart +
		`$`
	sampleParserSampleLineRE = regexp.MustCompile(sampleParserSampleLineREPart)
)

// parseSample reads a single sample/s description and converts it to set of samples
func parseSample(r io.Reader) ([]*sample, error) {
	var out []*sample

	scanner := bufio.NewScanner(r)

	kindMapper := func(symbol string) sampleKind {
		switch symbol {
		case string(sampleCounter):
			return sampleCounter
		case string(sampleGauge):
			return sampleGauge
		case string(sampleHistogramLinear):
			return sampleHistogramLinear
		}
		return sampleUnknown
	}

	labelsMapper := func(s string, out map[string]string) {
		for _, labelWithValue := range strings.Split(s, sampleParserLabelsSeparator) {
			// expecting always 2 values. It's enforced by earlier regexp check
			labelWithValueSlice := strings.SplitN(labelWithValue, sampleParserLabelFromValueSeparator, 2)
			out[labelWithValueSlice[0]] = labelWithValueSlice[1]
		}
	}

	isSampleLine := func(s string) bool {
		return sampleParserSampleLineRE.MatchString(s)
	}

	parseSampleLine := func(s string, sharedLabels map[string]string) *sample {
		samplePartsSlice := strings.Split(s, sampleParserSamplePartsSeparator)

		labels := make(map[string]string)
		for k, v := range sharedLabels {
			labels[k] = v
		}

		smp := sample{
			name:   samplePartsSlice[0],
			kind:   kindMapper(samplePartsSlice[1]),
			labels: labels,
		}
		smp.value, _ = strconv.ParseFloat(samplePartsSlice[len(samplePartsSlice)-1], 10)

		switch smp.kind {
		case sampleHistogramLinear:
			smp.histogramDef = strings.Split(samplePartsSlice[2], sampleParserHistogramDefSeparator)
			// account for histogramDef
			if len(samplePartsSlice) == 5 {
				labelsMapper(samplePartsSlice[3], smp.labels)
			}
		default:
			if len(samplePartsSlice) == 4 {
				labelsMapper(samplePartsSlice[2], smp.labels)
			}
		}

		return &smp
	}

	state := sampleParserStateSearching
	sharedLabels := make(map[string]string)

	for scanner.Scan() {
		switch state {
		case sampleParserStateSearching:
			if sampleParserSharedLabelsLineRE.MatchString(scanner.Text()) {
				sharedLabels = make(map[string]string) // reset
				labelsMapper(scanner.Text(), sharedLabels)
				state = sampleParserStateSample
				continue
			}

			if isSampleLine(scanner.Text()) {
				out = append(out, parseSampleLine(scanner.Text(), sharedLabels))
				continue
			}

		case sampleParserStateSample:
			if isSampleLine(scanner.Text()) {
				out = append(out, parseSampleLine(scanner.Text(), sharedLabels))
				continue
			}
		}
	}

	return out, nil
}
