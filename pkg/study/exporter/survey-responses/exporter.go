package surveyresponses

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	studytypes "github.com/case-framework/case-backend/pkg/study/types"
)

type ResponseExporter struct {
	parser    *ResponseParser
	writer    io.Writer
	csvWriter *csv.Writer
	format    string
	counter   int
}

func NewResponseExporter(
	parser *ResponseParser,
	writer io.Writer,
	format string,
) (*ResponseExporter, error) {
	re := &ResponseExporter{
		parser: parser,
		writer: writer,
		format: format,
	}

	if err := re.init(); err != nil {
		return nil, err
	}

	re.counter = 0

	return re, nil
}

func (re *ResponseExporter) init() error {
	var err error
	switch re.format {
	case "wide":
		re.csvWriter = csv.NewWriter(re.writer)
		record := []string{}
		record = append(record, re.parser.columns.FixedColumns...)
		record = append(record, re.parser.columns.ContextColumns...)
		record = append(record, re.parser.columns.ResponseColumns...)
		record = append(record, re.parser.columns.MetaColumns...)
		err = re.csvWriter.Write(record)
		if err != nil {
			return err
		}
	case "long":
		re.csvWriter = csv.NewWriter(re.writer)
		record := []string{}
		record = append(record, re.parser.columns.FixedColumns...)
		record = append(record, re.parser.columns.ContextColumns...)
		record = append(record, "responseSlot")
		record = append(record, "value")
		err = re.csvWriter.Write(record)
		if err != nil {
			return err
		}
	case "json":
		_, err = re.writer.Write([]byte("{ \"responses\": ["))
	default:
		return fmt.Errorf("unsupported format: %s", re.format)
	}
	return err
}

func (re *ResponseExporter) WriteResponse(
	rawResp *studytypes.SurveyResponse,
	trackingInfo ...AccountTrackingInfo,
) error {

	if re.parser == nil {
		return fmt.Errorf("parser not initialized")
	}
	if re.writer == nil {
		return fmt.Errorf("writer not initialized")
	}

	parsedResp, err := re.parser.ParseResponse(rawResp, trackingInfo...)
	if err != nil {
		return err
	}

	switch re.format {
	case "wide":
		cells, err := re.parser.ResponseToStrList(parsedResp)
		if err != nil {
			return err
		}
		err = re.csvWriter.Write(cells)
		if err != nil {
			return err
		}
	case "long":
		records, err := re.parser.ResponseToLongFormat(parsedResp)
		if err != nil {
			return err
		}
		for _, record := range records {
			err = re.csvWriter.Write(record)
			if err != nil {
				return err
			}
		}
	case "json":
		// write to json
		flatObj, err := re.parser.ResponseToFlatObj(parsedResp)
		if err != nil {
			return err
		}
		rV, err := json.Marshal(flatObj)
		if err != nil {
			return err
		}
		if re.counter > 0 {
			_, err = re.writer.Write([]byte(","))
			if err != nil {
				return err
			}
		}
		_, err = re.writer.Write(rV)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format: %s", re.format)
	}

	re.counter += 1

	return nil
}

func (re *ResponseExporter) Finish() error {
	switch re.format {
	case "wide":
		re.csvWriter.Flush()
	case "long":
		re.csvWriter.Flush()
	case "json":
		_, err := re.writer.Write([]byte("]}"))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format: %s", re.format)
	}
	return nil
}
