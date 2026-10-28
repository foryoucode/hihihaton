import axios from 'axios';

const api = axios.create();

if (import.meta.env.VITE_DEMO === 'true') {
    api.defaults.adapter = async config => {
        let { default: adapter } = await import('./demo/adapter.js');
        return adapter(config);
    };
}

export default api;
