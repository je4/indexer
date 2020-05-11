# indexer

based on the idea of https://github.com/dla-marbach/indexer the go indexer 
can be used to extract metadata from files while speeding up the whole process 
of the identification cascade

## Installation
    go get github.com/je4/indexer
    go build github.com/je4/indexer/cmd/identify
    
## Usage
### Start service

    identify -cfg indexer.toml

### Query service
    curl -X POST --data-binary @query.json http://localhost:81
    
query.json:

    {
      "url": "https://upload.wikimedia.org/wikipedia/commons/thumb/5/54/Stift_Melk_Nordseite_01.jpg/750px-Stift_Melk_Nordseite_01.jpg",
      "actions": ["siegfried","identify","ffprobe","tika"],
      "downloadmime": "^image/.*$",
      "headersize": 5000
    }

### JSON-Fields
* **url**: mandatory field (file:///...)
* **actions**: optional field, list of identifiers to use
* **downlaodmime**: optional field, regexp of mimetypes, which should be downloaded completely
* **headersize**: optional field, size of header which is downloaded for format recognition 
    
## Rights
Copyright 2020 Jürgen Enge, info-age GmbH, Basel

Licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0)