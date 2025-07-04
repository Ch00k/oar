package output

//import (
//    "bytes"
//    "os"
//    "testing"

//    "github.com/fatih/color"
//)

//func TestColorFunctions(t *testing.T) {
//    tests := []struct {
//        name          string
//        colorsEnabled bool
//        expectColored bool
//    }{
//        {
//            name:          "colors disabled",
//            colorsEnabled: false,
//            expectColored: false,
//        },
//        {
//            name:          "colors enabled",
//            colorsEnabled: true,
//            expectColored: true,
//        },
//    }

//    for _, tt := range tests {
//        t.Run(tt.name, func(t *testing.T) {
//            // Save original NoColor state
//            originalNoColor := color.NoColor

//            // Restore original NoColor state after test
//            defer func() { color.NoColor = originalNoColor }()

//            // Set test state
//            color.NoColor = !tt.colorsEnabled

//            // Re-initialize colors for this test
//            InitColors(false)

//            // Test success color function
//            successResult := maybeColorize(Success, "test message")
//            if tt.expectColored {
//                // Should contain ANSI color codes
//                if len(successResult) <= len("test message") {
//                    t.Errorf("Expected colored output to be longer than plain text")
//                }
//            } else {
//                // Should be plain text
//                if successResult != "test message" {
//                    t.Errorf("Expected plain text, got: %q", successResult)
//                }
//            }

//            // Test warning color function
//            warningResult := maybeColorize(Warning, "test message")
//            if tt.expectColored {
//                // Should contain ANSI color codes
//                if len(warningResult) <= len("test message") {
//                    t.Errorf("Expected colored output to be longer than plain text")
//                }
//            } else {
//                // Should be plain text
//                if warningResult != "test message" {
//                    t.Errorf("Expected plain text, got: %q", successResult)
//                }
//            }

//            // Test error color function
//            errorResult := maybeColorize(Error, "error message")
//            if tt.expectColored {
//                // Should contain ANSI color codes
//                if len(errorResult) <= len("error message") {
//                    t.Errorf("Expected colored output to be longer than plain text")
//                }
//            } else {
//                // Should be plain text
//                if errorResult != "error message" {
//                    t.Errorf("Expected plain text, got: %q", errorResult)
//                }
//            }
//        })
//    }
//}

//func TestColorFunctionsWithFormatting(t *testing.T) {
//    // Save original NoColor state
//    originalNoColor := color.NoColor

//    // Restore original NoColor state after test
//    defer func() { color.NoColor = originalNoColor }()

//    // Test with colors disabled for consistent output
//    color.NoColor = true

//    InitColors(false)

//    tests := []struct {
//        name     string
//        format   string
//        args     []any
//        expected string
//    }{
//        {
//            name:     "simple string",
//            format:   "hello",
//            args:     nil,
//            expected: "hello",
//        },
//        {
//            name:     "format with string",
//            format:   "hello %s",
//            args:     []any{"world"},
//            expected: "hello world",
//        },
//        {
//            name:     "format with multiple args",
//            format:   "project %s (ID: %s)",
//            args:     []any{"test", "123"},
//            expected: "project test (ID: 123)",
//        },
//        {
//            name:     "format with integer",
//            format:   "count: %d",
//            args:     []any{42},
//            expected: "count: 42",
//        },
//    }

//    for _, tt := range tests {
//        t.Run(tt.name, func(t *testing.T) {
//            successResult := maybeColorize(Success, tt.format, tt.args...)
//            if successResult != tt.expected {
//                t.Errorf("maybeColorize(Success) = %q, want %q", successResult, tt.expected)
//            }

//            warningResult := maybeColorize(Warning, tt.format, tt.args...)
//            if warningResult != tt.expected {
//                t.Errorf("maybeColorize(Warning) = %q, want %q", successResult, tt.expected)
//            }

//            errorResult := maybeColorize(Error, tt.format, tt.args...)
//            if errorResult != tt.expected {
//                t.Errorf("maybeColorize(Error) = %q, want %q", errorResult, tt.expected)
//            }
//        })
//    }
//}

//func TestPrintFunctions(t *testing.T) {
//    // Save original NoColor state
//    originalNoColor := color.NoColor

//    // Restore original NoColor state after test
//    defer func() { color.NoColor = originalNoColor }()

//    // Test with colors disabled for consistent output
//    color.NoColor = true

//    InitColors(false)

//    t.Run("printSuccess", func(t *testing.T) {
//        // Capture stdout
//        oldStdout := os.Stdout
//        r, w, _ := os.Pipe()
//        os.Stdout = w

//        PrintMessage(Success, "test success")

//        w.Close() // nolint: errcheck
//        os.Stdout = oldStdout

//        var buf bytes.Buffer
//        buf.ReadFrom(r) // nolint: errcheck

//        got := buf.String()
//        want := "test success\n"

//        if got != want {
//            t.Errorf("printSuccess() = %q, want %q", got, want)
//        }
//    })

//    t.Run("printWarning", func(t *testing.T) {
//        // Capture stdout
//        oldStdout := os.Stdout
//        r, w, _ := os.Pipe()
//        os.Stdout = w

//        PrintMessage(Warning, "test warning")

//        w.Close() // nolint:errcheck
//        os.Stdout = oldStdout

//        var buf bytes.Buffer
//        buf.ReadFrom(r) // nolint:errcheck

//        got := buf.String()
//        want := "test warning\n"

//        if got != want {
//            t.Errorf("printWarning() = %q, want %q", got, want)
//        }
//    })

//    t.Run("printError", func(t *testing.T) {
//        // Capture stderr
//        oldStdout := os.Stdout
//        r, w, _ := os.Pipe()
//        os.Stdout = w

//        PrintMessage(Error, "test error")

//        w.Close() // nolint: errcheck
//        os.Stdout = oldStdout

//        var buf bytes.Buffer
//        buf.ReadFrom(r) // nolint: errcheck

//        got := buf.String()
//        want := "test error\n"

//        if got != want {
//            t.Errorf("printError() = %q, want %q", got, want)
//        }
//    })
//}
