import tapoPlugApi
from flask import Flask, jsonify
from prometheus_client import Gauge, generate_latest
import json
import os

app = Flask(__name__)

plug_current_power_gauge = Gauge('plug_current_power', 'Plug Current Power', labelnames=['plug_name', 'plug_ip'])
plug_signal_level_gauge = Gauge('plug_signal_level', 'Plug Signal Level', labelnames=['plug_name', 'plug_ip'])
plug_rssi_gauge = Gauge('plug_rssi', 'Plug RSSI', labelnames=['plug_name', 'plug_ip'])
plug_status_gauge = Gauge('plug_status', 'Plug Status', labelnames=['plug_name', 'plug_ip'])
plug_overheated_gauge = Gauge('plug_overheated', 'Plug Overheated', labelnames=['plug_name', 'plug_ip'])
plug_today_runtime_gauge = Gauge('plug_today_runtime', 'Plug Today Runtime', labelnames=['plug_name', 'plug_ip'])
plug_month_runtime_gauge = Gauge('plug_month_runtime', 'Plug Month Runtime', labelnames=['plug_name', 'plug_ip'])
plug_on_time_gauge = Gauge('plug_on_time', 'Plug On Time', labelnames=['plug_name', 'plug_ip'])

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
    for pn, p in plugs.items():
        energy_usage_result = json.loads(tapoPlugApi.getEnergyUsageInfo(p))['result']
        #plug_usage_result = json.loads(tapoPlugApi.getPlugUsage(p))['result']
        device_running_result = json.loads(tapoPlugApi.getDeviceRunningInfo(p))['result']

        #print(device_running_result)
        #print(energy_usage_result)
        #print(plug_usage_result)

        current_power = energy_usage_result['current_power']
        plug_current_power_gauge.labels(plug_name=pn, plug_ip=p['tapoIp']).set(current_power)

        signal_level = device_running_result['signal_level']
        plug_signal_level_gauge.labels(plug_name=pn, plug_ip=p['tapoIp']).set(signal_level)

        rssi = device_running_result['rssi']
        plug_rssi_gauge.labels(plug_name=pn, plug_ip=p['tapoIp']).set(rssi)

        status = 1 if device_running_result['device_on'] else 0
        plug_status_gauge.labels(plug_name=pn, plug_ip=p['tapoIp']).set(status)

        overheated = 1 if device_running_result['overheated'] else 0
        plug_overheated_gauge.labels(plug_name=pn, plug_ip=p['tapoIp']).set(overheated)

        today_runtime = energy_usage_result['today_runtime']
        plug_today_runtime_gauge.labels(plug_name=pn, plug_ip=p['tapoIp']).set(today_runtime)

        month_runtime = energy_usage_result['month_runtime']
        plug_month_runtime_gauge.labels(plug_name=pn, plug_ip=p['tapoIp']).set(month_runtime)

        on_time = device_running_result['on_time']
        plug_on_time_gauge.labels(plug_name=pn, plug_ip=p['tapoIp']).set(on_time)

    return generate_latest()


if __name__ == "__main__":
    app.run(debug=True)
