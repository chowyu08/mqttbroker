FROM alpine
COPY broker /
COPY broker.json /
COPY tls /tls
COPY conf /conf

EXPOSE 1883
EXPOSE 1888
EXPOSE 1993

CMD ["/broker"]