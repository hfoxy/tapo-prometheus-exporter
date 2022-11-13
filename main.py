import tapoPlugApi
from flask import Flask, jsonify
from prometheus_client import Gauge, generate_latest
import json
import os

app = Flask(__name__)

plug_current_power_gauge = Gauge('plug_current_power', 'Plug Current Power', labelnames=['plug'])

tapo_email = os.getenv('TAPO_EMAIL')
tapo_password = os.getenv('TAPO_PASSWORD')

if tapo_email is None or tapo_password is None:
    raise Exception('login credentials not defined')

plug_defines = os.getenv('TAPO_PLUGS')
if plug_defines is None or plug_defines == '':
    raise Exception('plug(s) not defined')

print(f"Plug Defines: {plug_defines}")

for plug_data in plug_defines.split(','):
    plug_data_split = plug_data.split(':')
    plug_name = plug_data_split[0]
    plug_ip = plug_data_split[1]
    print(f"Plug: Name[{plug_name}] IP[{plug_ip}]")

    plug = {
        'tapoIp': plug_ip,
        'tapoEmail': tapo_email,
        'tapoPassword': tapo_password
    }

    print(f"Plug: {plug}")
    plug_current_power_gauge.labels(plug=plug_name).set_function(lambda: json.loads(
        tapoPlugApi.getEnergyUsageInfo(plug)
    )['result']['current_power'])
    print(generate_latest())


@app.route('/')
def main():
    result = {"status": "ok"}
    return jsonify(result)


@app.route('/metrics')
def metrics():
    return generate_latest()
