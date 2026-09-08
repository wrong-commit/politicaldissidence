#!/bin/sh
cd /Users/quinn/Home\ Junk/Personal/4.Development/3.Friday\ Night\ Projects/3.PoliticalDissidence
go build
if [ $? == 0 ]; then 
	./politicaldissidence
fi 
