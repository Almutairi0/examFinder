A fast, terminal-based utility written in Go designed to scrape, filter, and extract specific exam schedules from a raw university PDF file.

> To run this project, download the latest exam schedule PDF, and place it in the root folder.

**List of improvements Coming soon**

1. Persistent Configuration File

Allow the tool to read a configuration file (like config.toml, .json, or .yaml) stored in a standard location like ~/.config/examfinder/config.toml.

2. Upgrade to a True CLI with Flags

```

examfinder --file Exams.pdf --courses MATH101,CS211 --format ics

```


