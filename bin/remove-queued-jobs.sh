token=$GH_TOKEN
repo=brevdev/brev-cli

ids=$(curl -s -H "Authorization: token $token" \
    https://api.github.com/repos/$repo/actions/runs | \
    jq '.workflow_runs[] | select([.status] | inside(["in_progress", "queued"])) | .id')
set -- $ids
for i; do curl \
    -H "Authorization: token $token" \
    -X POST "https://api.github.com/repos/$repo/actions/runs/$i/cancel"; done
