
aws dynamodb batch-write-item \
    --request-items file://data/unit.data.new.json \
    --return-consumed-capacity TOTAL