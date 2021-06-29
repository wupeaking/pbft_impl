import Vue from 'vue';
// import Button from 'ant-design-vue/lib/button';
import Antd from "ant-design-vue";
import 'ant-design-vue/dist/antd.css';
import App from './App';

// Vue.component(Button.name, Button);
Vue.use(Antd);

Vue.config.productionTip = false;

new Vue({
  render: h => h(App),
}).$mount('#app');