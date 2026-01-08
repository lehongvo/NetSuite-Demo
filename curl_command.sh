#!/bin/bash
curl --location 'https://9342705.suitetalk.api.netsuite.com/services/rest/query/v1/suiteql?limit=1000&offset=100000' \
  --header 'Prefer: transient' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: OAuth realm="9342705",oauth_consumer_key="6cd361314fff88eec57bcb3c614cc756dbe04a0e89d2a3ab66e6c64c3dc82737",oauth_token="dbc891d00323d50d09ff761b5d91249332beb521fcb7c7fc83376f60682210ad",oauth_signature_method="HMAC-SHA256",oauth_timestamp="1767667313",oauth_nonce="1767667313126529000",oauth_version="1.0",oauth_signature="0B%2FsUXmy%2FRv5DW15ArQr3NeIOyJYy09kvVUfTzSB8WA%3D"' \
  --data '{
    "q": "SELECT il.*, i.itemid, l.name as locationname, TO_CHAR( il.lastquantityavailablechange, '"'"'YYYY-MM-DD HH24:MI:SS TZH:TZM'"'"') AS updatedat FROM item i, inventoryitemlocations il, location l WHERE i.isinactive = '"'"'F'"'"' AND i.id = il.item AND il.location = l.id "
}'
