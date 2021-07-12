# WebMapMaker
WebMapMaker is a simple tool that crawls your website to build its sitemap in xml or txt format.

## CLI usage
Build the binary with **make build**, then execute it with the following arguments:
```
./bin/makemap -t="https://example.com" -o="out.xml"
```
where **-t** is the target website from which to start crawling and **-o** is the output file (the file extension is required and must be either .xml or .txt) 
Other available arguments:
* **-mr** (max routines) - specify the maximum amount of goroutines running at the same time (by default, goroutines will be spawned for each page)
* **-so** (search options) - flags to configure the behavior of cralwer, specified as string separated by commas. The following flags can be set:
    * **ignoreTopLevelDomain** - when this options is set, pages with different top level domains will be included in the results. For example, if your website is foobarbaz.com and it has links to foobarbaz.es or foobarbaz.ru, they will also be included.
    * **includeWithQuery** - by default, all links with query strings will be ignored. This options allows to visit such links as well.
    * **includeSubdomains** - when this options is set, pages on subdomains will be included in the results. For example, if the initial domain is foo.com, links to domains bar.foo.com or baz.foo.com will be crawled. 

## Known issues:
- [] The tool never exits on some websites;
- [] CLI progress bar prints new frames on new line instead of rewriting old one when the output does not fit in one line in terminal window; 
