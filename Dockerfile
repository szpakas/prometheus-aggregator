FROM golang:1-onbuild
MAINTAINER Adam Szpakowski <adam@szpakowski.info>

# ingress: samples via UDP
EXPOSE 9991
# egress: metrics for prometheus to scrape
EXPOSE 8080
