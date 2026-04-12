import { Component, h } from "preact";
import './Announcement.css';
import { GetAnnouncement } from "../../wailsjs/go/main/App";

// 定义状态的类型
interface AnnouncementState {
    content: string;
}

export class Announcement extends Component<{}, AnnouncementState> {
    constructor() {
        super();
        // 初始化状态
        this.state = { content: "" };
    }

    // 生命周期：在组件创建时调用
    async componentDidMount() {
        try {
            const content = await GetAnnouncement(); // 确保使用 await 来处理异步调用
            this.setState({ content }); // 更新状态
        } catch (error) {
            console.error("获取公告失败:", error); // 处理错误
            this.setState({ content: "<p>无法获取公告内容</p>" }); // 可选：设置默认错误消息
        }
    }

    render() {
        return (
            <div id="Announcement" dangerouslySetInnerHTML={{ __html: this.state.content }}></div>
        );
    }
}
