# AdguardHome Stats Exporter
first golang "application"  
collecting dns queries, upstream RT, blocked dns queries, processing RT
```shell
docker run -it -p "8000:8000" \
  -e ADGUARD_ENDPOINT="" \
  -e ADGUARD_USERNAME="" \
  -e ADGUARD_PASSWORD="" \
  -e ADGUARD_PATH="/metrics" \
  -e ADGUARD_ADDRESS=":8000" \
  --name adguard-exporter \
  0x49f/adguardhome-exporter:v1.0
```