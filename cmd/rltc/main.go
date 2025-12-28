package main

import (
    "bytes"
    "encoding/hex"
    "flag"
    "fmt"
    "io"
    "os"
    "strings"

    intr "github.com/dadrian/relish/internal"
    "github.com/dadrian/relish/textrep"
)

func main() {
    in := flag.String("in", "-", "input .rlt file (or - for stdin)")
    out := flag.String("out", "-", "output file (or - for stdout)")
    hexOut := flag.Bool("hex", false, "write hex-encoded Relish bytes instead of binary")
    validate := flag.Bool("validate", false, "validate only; parse and encode without writing output")
    info := flag.Bool("info", false, "print a brief TLV summary (no output bytes)")
    flag.Parse()

    // Read input
    var inBytes []byte
    var err error
    if *in == "-" {
        inBytes, err = io.ReadAll(os.Stdin)
        if err != nil { fatalf("read stdin: %v", err) }
    } else {
        inBytes, err = os.ReadFile(*in)
        if err != nil { fatalf("read input: %v", err) }
    }

    // Parse + encode to bytes (validates syntax + mapping)
    outBytes, err := textrep.EncodeBytes(inBytes)
    if err != nil { fatalf("encode: %v", err) }

    if *info {
        if err := printInfo(outBytes); err != nil { fatalf("info: %v", err) }
        return
    }

    if *validate {
        // Validation-only: success => exit 0, no output
        return
    }

    // Prepare writer
    var w io.Writer = os.Stdout
    var outfile *os.File
    if *out != "-" {
        outfile, err = os.Create(*out)
        if err != nil { fatalf("create output: %v", err) }
        defer outfile.Close()
        w = outfile
    }

    if *hexOut {
        enc := hex.NewEncoder(w)
        if _, err := enc.Write(outBytes); err != nil { fatalf("write hex: %v", err) }
        // add trailing newline for text output convenience
        if _, err := w.Write([]byte("\n")); err != nil { fatalf("write newline: %v", err) }
        return
    }

    if _, err := w.Write(outBytes); err != nil { fatalf("write: %v", err) }
}

func printInfo(b []byte) error {
    r := bytes.NewReader(b)
    t, err := intr.ReadType(r)
    if err != nil { return err }
    n, _, err := intr.ReadLen(r)
    if err != nil { return err }
    fmt.Printf("Type: 0x%02x\n", t)
    fmt.Printf("Length: %d\n", n)
    // If not a struct, we're done
    if t != 0x11 {
        return nil
    }
    payload := make([]byte, n)
    if err := intr.ReadFull(r, payload); err != nil { return err }
    pr := bytes.NewReader(payload)
    var ids []string
    for pr.Len() > 0 {
        b, err := pr.ReadByte()
        if err != nil { return err }
        ids = append(ids, fmt.Sprintf("%d", int(b)))
        // Skip value TLV
        tt, err := intr.ReadType(pr)
        if err != nil { return err }
        if fs, ok := intr.FixedSize(tt); ok {
            if fs > 0 {
                skip := make([]byte, fs)
                if err := intr.ReadFull(pr, skip); err != nil { return err }
            }
        } else {
            ln, _, err := intr.ReadLen(pr)
            if err != nil { return err }
            if ln > 0 {
                skip := make([]byte, ln)
                if err := intr.ReadFull(pr, skip); err != nil { return err }
            }
        }
    }
    fmt.Printf("Fields: %s\n", strings.Join(ids, ","))
    return nil
}

func fatalf(f string, args ...any) {
    fmt.Fprintf(os.Stderr, f+"\n", args...)
    os.Exit(1)
}
