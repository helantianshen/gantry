import { createApp } from 'vue'
import { Button, Empty, Form, Input, InputNumber, Modal, Pagination, Select, Textarea } from '@arco-design/web-vue'
import '@arco-design/web-vue/es/button/style/css.js'
import '@arco-design/web-vue/es/empty/style/css.js'
import '@arco-design/web-vue/es/form/style/css.js'
import '@arco-design/web-vue/es/input/style/css.js'
import '@arco-design/web-vue/es/input-number/style/css.js'
import '@arco-design/web-vue/es/message/style/css.js'
import '@arco-design/web-vue/es/modal/style/css.js'
import '@arco-design/web-vue/es/pagination/style/css.js'
import '@arco-design/web-vue/es/select/style/css.js'
import '@arco-design/web-vue/es/textarea/style/css.js'
import App from './App.vue'
import router from './router'
import './styles.css'

const app = createApp(App).use(router)
;[Button, Empty, Form, Input, InputNumber, Modal, Pagination, Select, Textarea].forEach((component) => app.use(component))
app.mount('#app')
