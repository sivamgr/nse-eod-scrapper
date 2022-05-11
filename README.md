# nse-eod-scrapper
This application downloads the NSE EOD Data and Keeps it up-to-date.

This application requires a storage volume to keep the downloaded NSE EOD data file

```docker run --name nseeodsync -v $PWD/nsecm:/opt sivamgr/nse-eod-sync:latest```
