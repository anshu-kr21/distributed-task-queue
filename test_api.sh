#!/bin/bash

# Test script for the Distributed Task Queue API

BASE_URL="http://localhost:8080"

echo "🧪 Testing Distributed Task Queue API"
echo "======================================"
echo ""

# Test 1: Submit a job
echo "📤 Test 1: Submit a job"
response=$(curl -s -X POST $BASE_URL/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "user123",
    "payload": "{\"task\": \"process_data\", \"data\": \"example\"}",
    "max_retries": 3
  }')
echo "Response: $response"
job_id=$(echo $response | grep -o '"id":"[^"]*' | cut -d'"' -f4)
echo "Job ID: $job_id"
echo ""

# Test 2: Submit job with idempotency key
echo "📤 Test 2: Submit job with idempotency key"
idem_key="test-key-$(date +%s)"
curl -s -X POST $BASE_URL/api/jobs \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"user456\",
    \"payload\": \"{\\\"task\\\": \\\"important_task\\\"}\",
    \"idempotency_key\": \"$idem_key\",
    \"max_retries\": 5
  }" | jq '.'
echo ""

# Test 3: Try to submit same job again (should return existing job)
echo "📤 Test 3: Submit duplicate job with same idempotency key"
curl -s -X POST $BASE_URL/api/jobs \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"user456\",
    \"payload\": \"{\\\"task\\\": \\\"important_task\\\"}\",
    \"idempotency_key\": \"$idem_key\",
    \"max_retries\": 5
  }" | jq '.'
echo ""

# Test 4: Check job status
if [ ! -z "$job_id" ]; then
  echo "🔍 Test 4: Check job status"
  sleep 2
  curl -s "$BASE_URL/api/jobs/status?id=$job_id" | jq '.'
  echo ""
fi

# Test 5: List all jobs
echo "📋 Test 5: List all jobs"
curl -s "$BASE_URL/api/jobs" | jq '. | length as $count | "Total jobs: \($count)"'
echo ""

# Test 6: List pending jobs
echo "📋 Test 6: List pending jobs"
curl -s "$BASE_URL/api/jobs?status=pending" | jq '.'
echo ""

# Test 7: List jobs by tenant
echo "📋 Test 7: List jobs by tenant"
curl -s "$BASE_URL/api/jobs?tenant_id=user123" | jq '.'
echo ""

# Test 8: Get metrics
echo "📊 Test 8: Get metrics"
curl -s "$BASE_URL/api/metrics" | jq '.'
echo ""

# Test 9: Rate limiting test
echo "🚦 Test 9: Rate limiting test (submit 12 jobs rapidly)"
for i in {1..12}; do
  response=$(curl -s -w "\n%{http_code}" -X POST $BASE_URL/api/jobs \
    -H "Content-Type: application/json" \
    -d "{
      \"tenant_id\": \"rate-test-user\",
      \"payload\": \"{\\\"job\\\": $i}\"
    }")
  status=$(echo "$response" | tail -n1)
  if [ "$status" = "429" ]; then
    echo "  Job $i: ❌ Rate limited (expected after 10th job)"
  else
    echo "  Job $i: ✅ Accepted"
  fi
done
echo ""

# Test 10: Concurrent job limit test
echo "🚦 Test 10: Concurrent job limit test (6 long jobs)"
echo "Note: This test requires workers to be processing jobs slowly"
for i in {1..6}; do
  response=$(curl -s -w "\n%{http_code}" -X POST $BASE_URL/api/jobs \
    -H "Content-Type: application/json" \
    -d "{
      \"tenant_id\": \"quota-test-user\",
      \"payload\": \"{\\\"long_job\\\": $i}\"
    }")
  status=$(echo "$response" | tail -n1)
  if [ "$status" = "429" ]; then
    echo "  Job $i: ⚠️  Quota exceeded (max 5 concurrent jobs)"
  else
    echo "  Job $i: ✅ Accepted"
  fi
  sleep 0.5
done
echo ""

echo "✅ All tests completed!"
echo ""
echo "🌐 Open http://localhost:8080 in your browser to see the dashboard"

