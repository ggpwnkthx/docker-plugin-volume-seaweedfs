FROM nginx:alpine
COPY ./entrypoint.sh /bin/entrypoint.sh
RUN chmod +x /bin/entrypoint.sh
ENTRYPOINT [ "/bin/entrypoint.sh" ]