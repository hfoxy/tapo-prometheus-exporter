import tapoPlugApi
from flask import Flask, jsonify
from prometheus_client import Gauge, generate_latest
import json
import os

app = Flask(__name__)

plug_current_power_gauge = Gauge('plug_current_power', 'Plug Current Power', labelnames=['plug_name', 'plug_ip'])

tapo_email = os.getenv('TAPO_EMAIL')
tapo_password = os.getenv('TAPO_PASSWORD')

if tapo_email is None or tapo_password is None:
    raise Exception('login credentials not defined')

plug_defines = os.getenv('TAPO_PLUGS')
if plug_defines is None or plug_defines == '':
    raise Exception('plug(s) not defined')

plugs = {}
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

    plugs[plug_name] = plug


@app.route('/')
def main():
    result = {"status": "ok"}
    return jsonify(result)


@app.route('/metrics')
def metrics():
    for pn, p in plugs:
        current_power = json.loads(tapoPlugApi.getEnergyUsageInfo(p))['result']['current_power']
        plug_current_power_gauge.labels(plug_name=pn, plug_ip=p['tapoIp']).set(current_power)

    return generate_latest()


if __name__ == "__main__":
    app.run(debug=True)
