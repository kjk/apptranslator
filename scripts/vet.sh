#!/bin/bash

go tool vet -printfuncs=httpErrorf:1,panicif:1,Noticef,Errorf .
