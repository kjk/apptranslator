#!/bin/bash

go tool vet -printfuncs=httpErrorf:1,fatalIf:1,Noticef,Errorf .
