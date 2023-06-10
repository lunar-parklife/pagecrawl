
# PageCrawl
---

A dead simply page crawler.

This is a toy project to play with Go.

## Configuring

This tool can be configured with an INI file. It has 3 sections:
- Log
- Network
- Output

### Log

Lets you configure the path and name of the log file.

- Path
The path to where the log file is WITHOUT trailing slash. Defaults to the current directory.

- Name
The name of the path (without timestamp or extension)

### Network

Currently only configures the contents of the 'From' header.

- From
The value of the 'From' header. You should set this to your email or preferred contact info.


### Output

Configures where the results should be sent to.

- Kind
The kind of sink to output to. Currently only supports stdout.

- Path
Doesn't do anything right now.
