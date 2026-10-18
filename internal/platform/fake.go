package platform

import "errors"

// FakeClipboard is an in-memory Clipboard for tests.
type FakeClipboard struct {
	content string
}

func NewFakeClipboard() *FakeClipboard {
	return &FakeClipboard{}
}

func (c *FakeClipboard) SetText(text string) error {
	c.content = text
	return nil
}

func (c *FakeClipboard) Text() (string, error) {
	return c.content, nil
}

// FakeFileDialogs is a canned-response FileDialogs for tests: queue up the
// responses a test expects the user to pick, in call order.
type FakeFileDialogs struct {
	OpenResponses []FakeFileResponse
	SaveResponses []FakeFileResponse

	OpenRequests []OpenFileRequest
	SaveRequests []SaveFileRequest
}

// FakeFileResponse is one canned answer for a FakeFileDialogs call.
type FakeFileResponse struct {
	Path string
	OK   bool
	Err  error
}

func NewFakeFileDialogs() *FakeFileDialogs {
	return &FakeFileDialogs{}
}

func (f *FakeFileDialogs) ShowOpen(req OpenFileRequest) (string, bool, error) {
	f.OpenRequests = append(f.OpenRequests, req)
	if len(f.OpenResponses) == 0 {
		return "", false, nil
	}
	resp := f.OpenResponses[0]
	f.OpenResponses = f.OpenResponses[1:]
	return resp.Path, resp.OK, resp.Err
}

func (f *FakeFileDialogs) ShowSave(req SaveFileRequest) (string, bool, error) {
	f.SaveRequests = append(f.SaveRequests, req)
	if len(f.SaveResponses) == 0 {
		return "", false, nil
	}
	resp := f.SaveResponses[0]
	f.SaveResponses = f.SaveResponses[1:]
	return resp.Path, resp.OK, resp.Err
}

// FakeODBCAdmin is a canned-response ODBCAdmin for tests.
type FakeODBCAdmin struct {
	Path         string
	Err          error
	OpenedArches []string
}

func NewFakeODBCAdmin() *FakeODBCAdmin {
	return &FakeODBCAdmin{}
}

func (f *FakeODBCAdmin) Open(targetArch string) (string, error) {
	f.OpenedArches = append(f.OpenedArches, targetArch)
	if f.Err != nil {
		return f.Path, f.Err
	}
	if f.Path == "" {
		return "", errors.New("fake odbc admin: no path configured")
	}
	return f.Path, nil
}
