token=$GH_TOKEN
repo=brevdev/brev-cli
# get ids of all queued github actions runs for the repo
ids=$(curl -s -H "Authorization: token $token" "https://api.github.com/repos/$repo/actions/runs?status=queued&per_page=100" | jq -r '.workflow_runs[].id')
set -- $ids
for i; do curl \
    -H "Authorization: token $token" \
    -X POST "https://api.github.com/repos/$repo/actions/runs/$i/cancel"; done
