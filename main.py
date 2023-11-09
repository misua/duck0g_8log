from venv import logger
from flask import Flask, jsonify, request
from flask_inject import inject
import redis
import time
import logging
from pythonjsonlogger import jsonlogger
import sys
import os
import subprocess



#init redis

class CustomJsonFormatter(jsonlogger.JsonFormatter):
    def process_log_record(self, log_record):
        log_record["type"] = "API"
        log_record["level"] = log_record["levelname"].lower()
        del log_record["levelname"]
        return super().process_log_record(log_record)

formatter = CustomJsonFormatter("%(asctime) %(filename) %(funcName) %(lineno) %(levelname) %(message)")
handler = logging.StreamHandler(sys.stdout)
handler.setFormatter(formatter)
logging.basicConfig(handlers=[handler], level=logging.INFO)

app = Flask(__name__)
r = redis.Redis(host='localhost', port=6379, db=0,username='sab')
default_rate_limit = 10 # default maximum number of requests allowed from one IP address

#@app.before_first_request
@inject('app')
def init_redis(app):
    global r
    try:
        r = redis.Redis(host='localhost', port=6379, db=0)
        r.ping()
    except redis.exceptions.ConnectionError as e:
        print("Redis connection error: ", str(e))
       # print("Starting Redis server")
        p = subprocess.Popen(['my-redis-container'], stdout=subprocess.PIPE)
        time.sleep(1)
        r = redis.Redis(host='localhost', port=6379, db=0)


users = [
    {
        "id": 1,
        "name": "John Doe",
        "email": "john.doe@example.com"
    },
    {
        "id": 2,
        "name": "Jane Doe",
        "email": "jane.doe@example.com"
    }
]


@app.route('/api/<path>', methods=['GET'])
def api_endpoint(path):
    ip_address = request.remote_addr
    logger = logging.getLogger()

    if not r.exists(ip_address):
        r.set(ip_address, 1)
    
    else:
        current_count = int(r.incr(ip_address))
        if current_count >= default_rate_limit:
            logger.error({"event": "RateLimitExceeded", "ip_address": ip_address}, extra={'stack_info': True})
            return 'Rate Limit Exceeded!', 429



    if path.startswith('users/'):
        try:
            user_id = int(path.split('/')[-1])
            user = users(user_id)
            response = {'message': f'Hello from {path}!'}
            logger.info({"event": "APIRequest", "path": path, "status": 200}, extra={'stack_info': True})
            return jsonify(response), 200
        except ValueError as e:
            return f"Invalid user id: {e}", 400
    
    
    elif path == 'products':
        print("hello products")
        return "Not Found", 404

    response = {'message': f'Hello from {path}!'}
    logger.info({"event": "APIRequest", "path": path, "status": 200}, extra={'stack_info': True})
    return jsonify(response), 200




@app.route('/api/rlimit', methods=['POST'])
def set_rate_limit():
    logger = logging.getLogger(__name__)
    limit = request.json.get('limit')
    if limit is not None:
        try:    
            global default_rate_limit
            default_rate_limit = int(limit)
            logger.info({"event": "SetRateLimit", "new_rate_limit": limit})
            return jsonify({"message": f"New rate limit has been set ({default_rate_limit} requests per second)"}), 200
        except ValueError:
            logger.error({"event": "InvalidInputForRateLimit", "input": str(request.json)})
            return jsonify({"error": "Invalid input for new rate limit"}), 400
    else:
        logger.warning({"event": "NoRateLimitProvidedInRequest"})   
        return jsonify({"error": "No rate limit provided in request body"}), 400


if __name__ == '__main__':
    app.run(debug=True, port=8000)
