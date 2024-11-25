// Copyright 2012 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"io"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/mkacmaz/mse-tools/ebml"
	remuxer "github.com/mkacmaz/mse-tools/mse_webm_remuxer"
	"github.com/mkacmaz/mse-tools/webm"
	"golang.org/x/net/websocket"
)

func main() {
	var minClusterDurationInMS int
	flag.IntVar(&minClusterDurationInMS, "cm", 250, "Minimum Cluster Duration (ms)")
	flag.Parse()

	if minClusterDurationInMS < 0 || minClusterDurationInMS > 30000 {
		log.Printf("Invalid minimum cluster duration\n")
		os.Exit(-1)
	}

	if len(flag.Args()) < 2 {
		log.Printf("Usage: %s [-cm <duration>] <infile> <outfile>\n", os.Args[0])
		return
	}

	var in *os.File = nil
	var err error = nil

	inputArg := flag.Arg(0)
	outputArg := flag.Arg(1)

	if inputArg == "-" {
		in = os.Stdin
	} else {
		in, err = os.Open(inputArg)
		checkError("Open input", err)
	}

	var out *ebml.Writer = nil
	if outputArg == "-" {
		out = ebml.NewNonSeekableWriter(io.WriteSeeker(os.Stdout))
	} else {
		if inputArg == outputArg {
			log.Printf("Input and output filenames can't be the same.\n")
			return
		}

		if strings.HasPrefix(outputArg, "ws://") {
			url, err := url.Parse(outputArg)
			checkError("Output url", err)

			origin := "http://localhost/"
			ws, err := websocket.Dial(url.String(), "", origin)
			checkError("WebSocket Dial", err)
			out = ebml.NewNonSeekableWriter(io.Writer(ws))
		} else {
			file, err := os.Create(outputArg)
			if err != nil {
				log.Printf("Failed to create '%s'; err=%s\n", outputArg, err.Error())
				os.Exit(1)
			}
			out = ebml.NewWriter(io.WriteSeeker(file))
		}
	}

	buf := [1024]byte{}
	c := remuxer.NewDemuxerClient(out, minClusterDurationInMS)

	typeInfo := map[int]int{
		ebml.IdHeader:      ebml.TypeBinary,
		webm.IdSegment:     ebml.TypeList,
		webm.IdInfo:        ebml.TypeBinary,
		webm.IdTracks:      ebml.TypeBinary,
		webm.IdCluster:     ebml.TypeList,
		webm.IdTimecode:    ebml.TypeUint,
		webm.IdSimpleBlock: ebml.TypeBinary,
	}

	parser := ebml.NewParser(ebml.GetListIDs(typeInfo), webm.UnknownSizeInfo(),
		ebml.NewElementParser(c, typeInfo))

	for done := false; !done; {
		bytesRead, err := in.Read(buf[:])
		if err == io.EOF || err == io.ErrClosedPipe {
			parser.EndOfData()
			done = true
			continue
		}

		if !parser.Append(buf[0:bytesRead]) {
			log.Printf("Parser error")
			done = true
			continue
		}
	}
}

func checkError(str string, err error) {
	if err != nil {
		log.Printf("Error: %s - %s\n", str, err.Error())
		os.Exit(-1)
	}
}
