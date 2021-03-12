# openshift-test-result-filter

This tool accepts a JUNIT file as input, and allows you to filter the results.

* Filter by test result (`all`, `failed`, `passed`, `skipped`)
* Filter by tag (e.g. `sig-storage`)
* Optionally show errors for failed tests
* Show GitHub links to test definitions in openshift/origin

## Building

Requires Go >= 1.16

```
$ git clone https://github.com/honza/openshift-test-result-filter
$ cd openshift-test-result-filter
$ make
$ ./openshift-test-result-filter
```

## Running

```
Usage of ./openshift-test-result-filter:
  -filename string
    	input junit file
  -origin-tree-path string
    	
  -result string
    	choices: all, skipped, failed, passed (default "all")
  -show-errors
    	
  -tag string
    	Tag, e.g. sig-storage
```
