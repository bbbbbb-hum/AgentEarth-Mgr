import sys
import os
import json
import asyncio
from datetime import datetime
import traceback
import requests
import logging

api_probe_dir = os.path.dirname(os.path.abspath(__file__))
sys.path.append(api_probe_dir)

global_logger = None
cached_config = None

TIMESTAMP_FORMAT = '%Y-%m-%d %H:%M:%S'

def setup_global_logger():
    """设置全局日志记录器"""
    global global_logger
    if global_logger is not None:
        return global_logger
    
    config = load_config()
    log_config = config.get('LogConfig', {}) if config else {}
    
    log_file_path = log_config.get('log_file_path')
    log_level_str = log_config.get('log_level', 'INFO').upper()
    log_level = getattr(logging, log_level_str, logging.INFO)
    console_format = log_config.get('console_format', '%(message)s')
    file_format = log_config.get('file_format', '%(asctime)s - %(levelname)s - %(message)s')
    
    global_logger = logging.getLogger('MCPBasicProber')
    global_logger.setLevel(log_level)
    
    if not global_logger.handlers:
        console_handler = logging.StreamHandler(sys.stdout)
        file_handler = logging.FileHandler(log_file_path, mode='a', encoding='utf-8')
        console_formatter = logging.Formatter(console_format)
        file_formatter = logging.Formatter(file_format)
        console_handler.setFormatter(console_formatter)
        file_handler.setFormatter(file_formatter)
        global_logger.addHandler(console_handler)
        global_logger.addHandler(file_handler)
    
    global_logger.propagate = False
    
    return global_logger

def load_config():
    """从配置文件加载API配置
    
    Returns:
        dict: 配置字典，如果加载失败则返回 None
    """
    global cached_config
    if cached_config is not None:
        return cached_config
    
    config_file = "config.json"
    config_path = "/opt/xlconfigs/AEMgr/pyutils/" + config_file
    
    try:
        with open(config_path, 'r', encoding='utf-8') as f:
            config = json.load(f)
        cached_config = config
        return config
    except Exception as e:
        print(f"加载配置文件失败: {str(e)}")
        return None

def get_enabled_services():
    """从API获取服务列表，使用display_name作为服务名称"""
    response = None
    logger = setup_global_logger()
    
    try:
        config = load_config()
        if not config:
            logger.error("配置加载失败，无法获取服务列表")
            return []
        
        # 获取服务列表API配置
        service_list_api = config.get('ServiceListApi')
        if not service_list_api or 'url' not in service_list_api:
            logger.error("服务列表API配置不存在或不完整")
            return []
        
        api_url = service_list_api['url']
        logger.info(f"尝试从API获取服务列表: {api_url}")
        
        # 发送GET请求获取服务列表
        response = requests.get(api_url, timeout=30)
        
        # 检查响应状态
        if response.status_code != 200:
            logger.error(f"获取服务列表失败: HTTP状态码 {response.status_code}")
            logger.error(f"响应内容: {response.text}")
            return []
        
        # 解析JSON响应
        data = response.json()
        
        # 提取服务信息列表
        services_data = []
        
        # 处理不同的数据格式
        if isinstance(data, dict):
            # 检查数据格式：data.list
            if 'data' in data and isinstance(data['data'], dict) and 'list' in data['data'] and isinstance(data['data']['list'], list):
                services_data = data['data']['list']
        elif isinstance(data, list):
            # 直接使用列表格式
            services_data = data
        else:
            logger.error(f"服务列表API返回数据格式异常: 期望字典或列表类型，实际类型 {type(data).__name__}")
            logger.error(f"原始响应数据: {data}")
            return []
        
        # 提取服务信息并根据server_id去重
        services = []
        seen_server_ids = set()  # 用于存储已见过的server_id
        
        for item in services_data:
            if isinstance(item, dict) and 'server_id' in item and 'display_name' in item:
                server_id = item['server_id']
                # 如果server_id未见过，则添加到结果列表并标记为已见
                if server_id not in seen_server_ids:
                    seen_server_ids.add(server_id)
                    services.append({
                        "server_id": server_id,
                        "server_name": item['display_name']  # 使用display_name作为服务名称
                    })
                else:
                    logger.warning(f"忽略重复服务: ID={server_id}, 名称={item['display_name']}")
        
        if services:
            logger.info(f"成功获取 {len(services)} 个服务")
        else:
            logger.warning("警告: 未能从API响应中提取到有效的服务信息")
        
        return services
        
    except requests.exceptions.RequestException as e:
        logger.error(f"获取服务列表网络错误: {str(e)}")
        logger.debug(f"堆栈跟踪:\n{traceback.format_exc()}")
        return []
    except json.JSONDecodeError as e:
        logger.error(f"解析服务列表响应失败: {str(e)}")
        logger.error(f"原始响应内容: {response.text[:200]}...")
        logger.debug(f"堆栈跟踪:\n{traceback.format_exc()}")
        return []
    except Exception as e:
        logger.error(f"获取服务列表失败: {str(e)}")
        logger.debug(f"堆栈跟踪:\n{traceback.format_exc()}")
        return []

def get_api_info():
    """获取API信息"""
    logger = setup_global_logger()
    
    config = load_config()
    if not config:
        logger.error("配置加载失败，无法获取API信息")
        return None
    
    api_info = config.get('ApiInfo', None)
    if not api_info:
        logger.error("API配置信息不存在")
        return None
    
    if 'X-API-Key' not in api_info or not api_info['X-API-Key']:
        logger.error("错误: API密钥不能为空")
        return None
    
    logger.info(f"API配置已加载: addr={api_info['addr']}")
    return api_info

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

class MCPBasicProber:
    
    def __init__(self):
        """初始化MCP一级探查器，设置初始状态"""
        self.services = []
        self.api_info = None
        self.probe_results = []
        
        # 使用全局日志记录器
        self.logger = setup_global_logger()
    
    def _handle_error(self, server_name, error):
        """统一的错误处理函数
        
        Args:
            server_name (str): 服务器名称
            error (Exception): 异常对象
            
        Returns:
            dict: 错误结果字典
        """
        error_type = type(error).__name__
        error_msg = str(error)
        error_traceback = traceback.format_exc()
        
        if isinstance(error, asyncio.TimeoutError):
            self.logger.error(f"连接服务 {server_name} 超时")
            self.logger.debug(f"堆栈跟踪:\n{error_traceback}")
            error_desc = "连接超时"
        elif isinstance(error, ConnectionError):
            self.logger.error(f"连接服务 {server_name} 失败: {error_msg}")
            self.logger.debug(f"堆栈跟踪:\n{error_traceback}")
            error_desc = f'连接错误: {error_msg}'
        else:
            self.logger.error(f"获取服务 {server_name} 工具定义时发生异常: {error_type} - {error_msg}")
            self.logger.debug(f"堆栈跟踪:\n{error_traceback}")
            error_desc = f'未知错误: {error_type} - {error_msg}'
        
        return self._create_result_dict(server_name, 'error', error=error_desc)
    
    def _build_request_headers(self, server_id):
        """构建服务URL和请求头
        
        Args:
            server_id (str): 服务器ID
            
        Returns:
            tuple: (server_url, headers) 或 (None, None) 如果构建失败
        """
        base_url = self.api_info.get('addr', '')
        if not base_url:
            self.logger.error("错误: 无法获取基础URL")
            return None, None
        
        server_url = f"{base_url}/mcp-server/{server_id}"
        headers = {
            'X-API-Key': self.api_info.get('X-API-Key', ''),
            'Content-Type': 'application/json'
        }
        
        return server_url, headers
    
    def _create_result_dict(self, server_name, status, error=None, payload=None):
        """创建统一的结果字典
        
        Args:
            server_name (str): 服务器名称
            status (str): 状态，'success' 或 'error'
            error (str, optional): 错误信息
            payload (dict, optional): 成功时的负载数据
            
        Returns:
            dict: 结果字典
        """
        result = {
            'server': server_name,
            'tool': 'tools_list',
            'status': status,
            'error': error,
            'timestamp': datetime.now().strftime(TIMESTAMP_FORMAT),
            'execution_time': 0,
            'has_valid_result': status == 'success'
        }
        
        if payload is not None:
            result['payload'] = payload
        
        return result
    
    def _validate_service_info(self, service):
        """验证服务信息的完整性
        
        Args:
            service (dict): 服务信息字典
            
        Returns:
            tuple: (is_valid, server_id, server_name)
        """
        server_id = service.get('server_id')
        server_name = service.get('server_name')
        
        if not server_id or not server_name:
            return False, None, None
        
        return True, server_id, server_name
    
    def initialize(self):
        """初始化探查器：加载配置、获取服务列表"""
        self.logger.info("开始初始化MCP接口一级探查器...")
        
        # 步骤1: 获取API信息
        self.logger.info("步骤1: 获取API信息")
        self.api_info = get_api_info()
        if not self.api_info:
            self.logger.error("错误: 无法获取API信息")
            return False
        
        # 验证API信息的关键字段
        if not all(k in self.api_info for k in ['addr', 'X-API-Key']):
            self.logger.error("错误: API信息不完整，缺少必要字段")
            return False
        self.logger.info(f"API信息获取成功，基础URL: {self.api_info['addr']}")
        
        # 步骤2: 从API获取服务列表
        self.logger.info("步骤2: 从API获取服务列表")
        self.services = get_enabled_services()
        if not self.services:
            self.logger.warning("警告: 未获取到任何服务")
        
        self.logger.info("MCP接口一级探查器初始化完成")
        return True
    
    async def _get_tools_only(self, server_name, server_url, headers):
        """只获取工具定义，不调用工具
        Args:
            server_name (str): 服务器名称
            server_url (str): 服务器URL
            headers (dict): HTTP请求头
        """
        self.logger.info(f"正在获取服务 {server_name} 的工具定义...")
        
        try:
            async with streamablehttp_client(server_url, headers=headers) as (read, write, _):
                async with ClientSession(read, write) as session:
                    await session.initialize()
                    
                    # 获取所有工具定义
                    tools_result = await session.list_tools()
                    
                    if hasattr(tools_result, 'tools') and tools_result.tools:
                        self.logger.info(f"成功获取 {len(tools_result.tools)} 个工具定义")
                        
                        # 添加到结果列表
                        payload = {
                            'tools_count': len(tools_result.tools),
                            'tools': [
                                {
                                    'name': tool.name,
                                    'description': tool.description if hasattr(tool, 'description') else '',
                                    'parameters': tool.schema if hasattr(tool, 'schema') else {}
                                }
                                for tool in tools_result.tools
                            ]
                        }
                        self.probe_results.append(self._create_result_dict(server_name, 'success', payload=payload))
                    else:
                        self.logger.error(f"无法获取服务 {server_name} 的工具定义")
                        self.probe_results.append(self._create_result_dict(server_name, 'error', error='无法获取工具定义列表'))
                        
        except (asyncio.TimeoutError, ConnectionError, Exception) as e:
            self.probe_results.append(self._handle_error(server_name, e))
    
    async def probe_service_tools(self, service):
        """探查单个服务的工具定义
        
        Args:
            service (dict): 服务信息字典，包含server_id和server_name
        """
        # 验证服务信息
        is_valid, server_id, server_name = self._validate_service_info(service)
        if not is_valid:
            self.logger.warning("警告: 服务信息不完整，跳过探查")
            return
            
        self.logger.info(f"开始一级探测服务: {server_name} (ID: {server_id})")
        
        # 构建服务URL和头部信息
        server_url, headers = self._build_request_headers(server_id)
        if not server_url or not headers:
            self.logger.error(f"错误: 无法构建请求信息，跳过服务 {server_name} 的探查")
            return
        
        # 获取工具定义
        await self._get_tools_only(server_name, server_url, headers)
    
    async def run_probe_cycle(self):
        """运行一次完整的一级探测周期"""
        self.logger.info("开始执行MCP接口一级探测周期")
        self.logger.info(f"开始时间: {datetime.now().strftime(TIMESTAMP_FORMAT)}")
        
        # 初始化探测状态
        self.probe_results = []
        
        # 检查是否有服务需要探测
        if not self.services:
            self.logger.error("错误: 没有可用的服务进行探测")
            self.print_probe_summary()
            return
        
        self.logger.info(f"准备探测 {len(self.services)} 个服务")
        
        # 探测每个服务
        for idx, service in enumerate(self.services, 1):
            self.logger.info(f"[{idx}/{len(self.services)}] 开始探测服务")
            await self.probe_service_tools(service)
        
        self.logger.info(f"探测周期结束")
        self.logger.info(f"结束时间: {datetime.now().strftime(TIMESTAMP_FORMAT)}")
        
        # 打印探测结果摘要
        self.print_probe_summary()
    
    def print_probe_summary(self):
        """打印一级探测结果摘要"""
        if not self.probe_results:
            self.logger.info("没有探测结果")
            return
            
        # 按服务分组结果
        service_results = {}
        for result in self.probe_results:
            server_name = result['server']
            if server_name not in service_results:
                service_results[server_name] = {
                    'status': 'unknown',
                    'tools': [],
                    'error': None
                }
            
            if result['tool'] == 'tools_list' and result['status'] == 'success':
                service_results[server_name]['status'] = 'success'
                if 'payload' in result and 'tools' in result['payload']:
                    service_results[server_name]['tools'] = result['payload']['tools']
            elif result['tool'] == 'tools_list' and result['status'] == 'error':
                service_results[server_name]['status'] = 'error'
                service_results[server_name]['error'] = result['error']
        
        # 统计结果
        total_services = len(service_results)
        successful_services = sum(1 for r in service_results.values() if r['status'] == 'success')
        failed_services = total_services - successful_services
        total_tools = sum(len(r['tools']) for r in service_results.values())
        
        # 计算成功率
        success_rate = (successful_services / total_services * 100) if total_services > 0 else 0
        
        # 测试完成统计
        self.logger.info("测试完成统计:")
        self.logger.info(f"总服务数: {total_services}个，连接成功: {successful_services}个，连接失败{failed_services}个，服务连接成功率{success_rate:.1f}%")
        self.logger.info(f"总工具数: {total_tools}个")
        
        # 成功的服务及其工具数
        if successful_services > 0:
            self.logger.info("连接成功的服务:")
            for server_name, result in service_results.items():
                if result['status'] == 'success':
                    self.logger.info(f"  - {server_name}: {len(result['tools'])} 个工具")
        
        # 失败的服务详情
        if failed_services > 0:
            self.logger.info("连接失败的服务:")
            for server_name, result in service_results.items():
                if result['status'] == 'error':
                    self.logger.info(f"  - {server_name}: {result.get('error', '未知错误')}")
    

    async def main(self):
        """主函数 - 执行一级探测器的初始化，并运行一次探测周期"""
        self.logger.info(f"{'='*60}")
        self.logger.info(f"启动时间: {datetime.now().strftime(TIMESTAMP_FORMAT)}")
        self.logger.info(f"{'='*60}")
        self.logger.info("MCP接口一级探测工具")
        self.logger.info("功能: 从API读取服务接口，连接服务并获取工具定义")
        
        if not self.initialize():
            self.logger.error("初始化失败，无法继续执行探测")
            return False
        
        self.logger.info("初始化成功，准备开始探测")
        
        await self.run_probe_cycle()
        
        self.logger.info("探测完成，程序退出")
        return True

# 主程序入口
if __name__ == "__main__":
    logger = setup_global_logger()
    logger.info("探测所有可用服务")
    
    prober = MCPBasicProber()
    
    try:
        asyncio.run(prober.main())
    except ImportError as e:
        logger.error(f"\n错误: 缺少必要的模块 - {str(e)}")
    except ConnectionError as e:
        logger.error(f"\n错误: 网络连接失败 - {str(e)}")
        logger.error("请检查网络连接和API地址设置")
    except Exception as e:
        logger.error(f"\n错误: 程序发生未预期的异常 - {str(e)}")
        logger.debug(f"堆栈跟踪:\n{traceback.format_exc()}")