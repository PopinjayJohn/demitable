import { mount } from 'svelte';
import App from './App.svelte';
import './lib/tokens.css';

const target = document.getElementById('app');
if (!target) throw new Error('missing #app');

const app = mount(App, { target });
export default app;
