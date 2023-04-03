FROM golang:1.19

WORKDIR /go/src/epaxos
COPY . .
# RUN go mod download
EXPOSE 0-65535
RUN apt-get update
RUN apt-get install -y vim zsh tmux net-tools openssh-server
RUN service ssh start
RUN chsh -s /bin/zsh
RUN sh -c "$(curl -fsSL https://raw.githubusercontent.com/robbyrussell/oh-my-zsh/master/tools/install.sh)"
RUN ./build.sh
WORKDIR /go/src/epaxos/bin
