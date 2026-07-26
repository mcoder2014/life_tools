import UlanziApi from './vendor/ulanzi-api/ulanziApi.js';
import { registerCommandPlugin } from './command-plugin.js';

const api = new UlanziApi();
registerCommandPlugin(api);
api.connect('com.ulanzi.ulanzistudio.commandexecutor');
