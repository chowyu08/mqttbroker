FROM alpine
COPY broker /
COPY broker.json /
COPY tls /tls

EXPOSE 1883
EXPOSE 1888
EXPOSE 1993

CMD ["/broker"]