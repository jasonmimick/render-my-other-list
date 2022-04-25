#!/usr/bin/env bash

NUM="${1:-10}"
URL="${2:-https://render-my-list.onrender.com/}"
echo "Test NUM=${NUM} URL=${URL}"
#seq ${NUM} | xargs -I {}
for i in $(seq ${NUM})
do
    X=$(printf '%x\n' $(($i+8969)));
    U="${URL}?i=an%20item%20${i}%20&p=${i}"
    echo curl "\"${U}\""
    curl "${U}"
done

